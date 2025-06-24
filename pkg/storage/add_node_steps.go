// Copyright 2025 Open3FS Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
	"github.com/open3fs/m3fs/pkg/task/steps"
	"github.com/open3fs/m3fs/pkg/utils"
)

type chainID = int64
type targetID = int64
type nodeID = int64
type diskIndex = int

type chainDistribution map[chainID]utils.Set[diskOnNode]

type diskOnNode struct {
	nodeID    nodeID
	diskIndex diskIndex
}

type fsChain struct {
	id        chainID
	targetIDs []targetID
}

type fsTarget struct {
	ID        targetID  `json:"id"`
	NodeID    nodeID    `json:"node_id"`
	DiskIndex diskIndex `json:"disk_index"`
	ChainID   chainID   `json:"chain_id"`
	status    string
}

type createTargetOpData struct {
	NodeID    nodeID    `json:"node_id"`
	TargetID  targetID  `json:"target_id"`
	DiskIndex diskIndex `json:"disk_index"`
	ChainID   chainID   `json:"chain_id"`
}

type addTargetToChainOpData struct {
	TargetID targetID `json:"target_id"`
	ChainID  chainID  `json:"chain_id"`
}

type offlineTargetOpData struct {
	TargetID targetID `json:"target_id"`
}

type removeTargetFromChainOpData struct {
	ChainID  chainID  `json:"chain_id"`
	TargetID targetID `json:"target_id"`
}

type removeTargetOpData struct {
	NodeID   nodeID   `json:"node_id"`
	TargetID targetID `json:"target_id"`
}

type uploadChainsOpData struct {
	TargetChainMapping map[targetID]chainID `json:"target_chain_mapping"`
	ChainsFile         string               `json:"chains_file"`
}

type uploadChainTableOpData struct {
	ChainTableID   int64  `json:"chain_table_id"`
	ChainTableFile string `json:"chain_table_file"`
}

// targetPool maintain assignable target
type targetPool struct {
	targetPrefix     int64
	currentTargetMap map[nodeID]map[diskIndex]*utils.Set[targetID]
}

func newTargetPool(targetPrefix int64, targets []fsTarget) *targetPool {
	tp := &targetPool{
		targetPrefix:     targetPrefix,
		currentTargetMap: make(map[nodeID]map[diskIndex]*utils.Set[targetID]),
	}

	for _, target := range targets {
		node := target.NodeID
		disk := target.DiskIndex
		index := target.ID
		if _, ok := tp.currentTargetMap[node]; !ok {
			tp.currentTargetMap[node] = make(map[diskIndex]*utils.Set[targetID])
		}
		if _, ok := tp.currentTargetMap[node][disk]; !ok {
			tp.currentTargetMap[node][disk] = utils.NewSet[targetID]()
		}
		tp.currentTargetMap[node][disk].Add(index)
	}
	return tp
}

func (tp *targetPool) GetAvailableTargetID(node nodeID, disk diskIndex) (targetID, error) {
	if _, has := tp.currentTargetMap[node]; !has {
		tp.currentTargetMap[node] = make(map[diskIndex]*utils.Set[targetID])
	}
	if _, has := tp.currentTargetMap[node][disk]; !has {
		tp.currentTargetMap[node][disk] = utils.NewSet[targetID]()
	}

	// Target index range is [1, 100)
	// Limitation from the gen_chain_table.py script in the 3fs project:
	//  https://github.com/deepseek-ai/3FS/blob/main/deploy/data_placement/src/setup/gen_chain_table.py#L94
	for index := 1; index < 100; index++ {
		id := ((tp.targetPrefix*1_000_000+node)*1_000+(int64(disk)+1))*100 + int64(index)
		if !tp.currentTargetMap[node][disk].Contains(id) {
			tp.currentTargetMap[node][disk].Add(id)
			return id, nil
		}
	}
	return 0, errors.Errorf("no available target id found in node %d and disk %d", node, disk)
}

func (tp *targetPool) Remove(node nodeID, disk diskIndex, targetID targetID) error {
	if _, has := tp.currentTargetMap[node]; !has {
		return nil
	}
	if _, has := tp.currentTargetMap[node][disk]; !has {
		return nil
	}
	tp.currentTargetMap[node][disk].Remove(targetID)
	return nil
}

type prepareChangePlanStep struct {
	task.BaseStep
}

func (s *prepareChangePlanStep) Execute(ctx context.Context) error {
	db := s.Runtime.LoadDB()
	var storServices []*model.StorService
	err := db.Model(new(model.StorService)).Find(&storServices).Error
	if err != nil {
		return errors.Trace(err)
	}
	storServiceNodeIDs := make([]uint, len(storServices))
	for i, storService := range storServices {
		storServiceNodeIDs[i] = storService.NodeID
	}
	storCfg := s.Runtime.Cfg.Services.Storage
	var newNodes []*model.Node
	err = db.Model(new(model.Node)).Find(&newNodes, "name IN (?) AND id NOT IN (?)",
		storCfg.Nodes, storServiceNodeIDs).Error
	if err != nil {
		return errors.Trace(err)
	}

	// re-calculate the cur data placement model
	modelPath, err := s.execDataPlacementScript(ctx, len(storServices), "")
	if err != nil {
		return errors.Annotate(err, "run data_placement.py to calculate the new model")
	}
	// calculate the new data placement model
	modelPath, err = s.execDataPlacementScript(ctx, len(storCfg.Nodes), modelPath)
	if err != nil {
		return errors.Annotate(err, "run data_placement.py to calculate the new model")
	}
	newDistribution, _, err := s.parseDataPlacementModel(ctx, modelPath)
	if err != nil {
		return errors.Annotate(err, "parse new data placement model")
	}
	curDistribution, curTargets, err := s.getCurrentDistribution()
	if err != nil {
		return errors.Trace(err)
	}
	steps, err := s.generateSteps(curDistribution, newDistribution, curTargets)
	if err != nil {
		return errors.Annotate(err, "generate change plan steps")
	}

	newNodeIDs := make([]uint, len(newNodes))
	for i, newNode := range newNodes {
		newNodeIDs[i] = newNode.ID
	}

	planData := map[string][]uint{
		"new_node_ids": newNodeIDs,
	}
	data, err := json.Marshal(planData)
	if err != nil {
		return errors.Trace(err)
	}
	changePlan := &model.ChangePlan{
		Type: model.ChangePlanTypeAddStorNodes,
		Data: string(data),
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		err = tx.Model(new(model.ChangePlan)).Create(changePlan).Error
		if err != nil {
			return errors.Trace(err)
		}
		for i := range steps {
			steps[i].ChangePlanID = changePlan.ID
		}
		err = tx.Model(new(model.ChangePlanStep)).Create(steps).Error
		if err != nil {
			return errors.Trace(err)
		}

		return nil
	})
	if err != nil {
		return errors.Trace(err)
	}

	s.Runtime.Store(task.RuntimeChangePlanKey, changePlan)
	s.Runtime.Store(task.RuntimeChangePlanStepsKey, steps)

	return nil
}

func (s *prepareChangePlanStep) generateSteps(
	currentDistribution chainDistribution,
	newDistribution chainDistribution,
	currentTargets []fsTarget,
) ([]*model.ChangePlanStep, error) {

	storCfg := s.Runtime.Cfg.Services.Storage
	targetPool := newTargetPool(storCfg.TargetIDPrefix, currentTargets)
	currentTargetMap := make(map[string]fsTarget)
	newTargetChainMap := make(map[targetID]chainID)
	for _, target := range currentTargets {
		// key = chainID:nodeID:diskIndex
		key := fmt.Sprintf("%d:%d:%d", target.ChainID, target.NodeID, target.DiskIndex)
		currentTargetMap[key] = target
		newTargetChainMap[target.ID] = target.ChainID
	}

	steps := []*model.ChangePlanStep{{OperationType: model.ChangePlanStepOpType.CreateStorService}}
	for chainID, currentNodes := range currentDistribution {
		newNodes, has := newDistribution[chainID]
		if !has || currentNodes.Equal(newNodes) {
			continue
		}

		needRemove := currentNodes.Difference(newNodes)
		needAdd := newNodes.Difference(currentNodes)

		for node := range needAdd {
			targetID, err := targetPool.GetAvailableTargetID(node.nodeID, node.diskIndex)
			if err != nil {
				return nil, errors.Trace(err)
			}

			createTarget := createTargetOpData{
				NodeID:    node.nodeID,
				TargetID:  targetID,
				DiskIndex: node.diskIndex,
				ChainID:   chainID,
			}
			createTargetData, err := utils.JsonMarshalString(createTarget)
			if err != nil {
				return nil, errors.Trace(err)
			}
			addTargetToChain := addTargetToChainOpData{
				TargetID: targetID,
				ChainID:  chainID,
			}
			addTargetToChainData, err := utils.JsonMarshalString(addTargetToChain)
			if err != nil {
				return nil, errors.Trace(err)
			}
			newTargetChainMap[targetID] = chainID

			steps = append(steps, &model.ChangePlanStep{
				OperationType: model.ChangePlanStepOpType.CreateTarget,
				OperationData: createTargetData,
			}, &model.ChangePlanStep{
				OperationType: model.ChangePlanStepOpType.AddTargetToChain,
				OperationData: addTargetToChainData,
			})
		}
		for node := range needRemove {
			key := fmt.Sprintf("%d:%d:%d", chainID, node.nodeID, node.diskIndex)
			target, ok := currentTargetMap[key]
			if !ok {
				return nil, errors.Errorf("target not found in node %d and disk %d",
					node.nodeID, node.diskIndex)
			}

			offlineTargetData, err := utils.JsonMarshalString(offlineTargetOpData{
				TargetID: target.ID,
			})
			if err != nil {
				return nil, errors.Trace(err)
			}
			removeTargetFromChainData, err := utils.JsonMarshalString(removeTargetFromChainOpData{
				ChainID:  chainID,
				TargetID: target.ID,
			})
			if err != nil {
				return nil, errors.Trace(err)
			}
			removeTargetData, err := utils.JsonMarshalString(removeTargetOpData{
				NodeID:   target.NodeID,
				TargetID: target.ID,
			})
			if err != nil {
				return nil, errors.Trace(err)
			}
			if err = targetPool.Remove(target.NodeID, target.DiskIndex, target.ID); err != nil {
				return nil, errors.Trace(err)
			}

			delete(newTargetChainMap, target.ID)
			steps = append(steps, &model.ChangePlanStep{
				OperationType: model.ChangePlanStepOpType.OfflineTarget,
				OperationData: offlineTargetData,
			}, &model.ChangePlanStep{
				OperationType: model.ChangePlanStepOpType.RemoveTargetFromChain,
				OperationData: removeTargetFromChainData,
			}, &model.ChangePlanStep{
				OperationType: model.ChangePlanStepOpType.RemoveTarget,
				OperationData: removeTargetData,
			})
		}
	}

	var createTargetDatas []createTargetOpData
	for chainID, diskOnNodeSet := range newDistribution {
		if _, has := currentDistribution[chainID]; has {
			continue
		}
		for diskOnNode := range diskOnNodeSet {
			targetID, err := targetPool.GetAvailableTargetID(diskOnNode.nodeID, diskOnNode.diskIndex)
			if err != nil {
				return nil, errors.Trace(err)
			}
			createTargetDatas = append(createTargetDatas, createTargetOpData{
				NodeID:    diskOnNode.nodeID,
				TargetID:  targetID,
				DiskIndex: diskOnNode.diskIndex,
				ChainID:   chainID,
			})

			newTargetChainMap[targetID] = chainID
		}
	}
	// NOTE: sort create target data by target id order so we can check it in ut
	sort.Slice(createTargetDatas, func(i, j int) bool {
		return createTargetDatas[i].TargetID < createTargetDatas[j].TargetID
	})
	for _, createTargetData := range createTargetDatas {
		data, err := utils.JsonMarshalString(createTargetData)
		if err != nil {
			return nil, errors.Trace(err)
		}
		steps = append(steps, &model.ChangePlanStep{
			OperationType: model.ChangePlanStepOpType.CreateTarget,
			OperationData: data,
		})
	}

	updateChainsData, err := utils.JsonMarshalString(uploadChainsOpData{
		TargetChainMapping: newTargetChainMap,
		ChainsFile:         "output/generated_chains.csv",
	})
	if err != nil {
		return nil, errors.Trace(err)
	}
	steps = append(steps, &model.ChangePlanStep{
		OperationType: model.ChangePlanStepOpType.UploadChains,
		OperationData: updateChainsData,
	})

	uploadChainTableData, err := utils.JsonMarshalString(uploadChainTableOpData{
		ChainTableID:   1,
		ChainTableFile: "output/generated_chain_table.csv",
	})
	if err != nil {
		return nil, errors.Trace(err)
	}
	steps = append(steps, &model.ChangePlanStep{
		OperationType: model.ChangePlanStepOpType.UploadChainTable,
		OperationData: uploadChainTableData,
	}, &model.ChangePlanStep{
		OperationType: model.ChangePlanStepOpType.SyncChainAndTargetModel,
	})

	return steps, nil
}

func (s *prepareChangePlanStep) getCurrentDistribution() (chainDistribution, []fsTarget, error) {
	db := s.Runtime.LoadDB()
	var chains []model.Chain
	err := db.Model(new(model.Chain)).Find(&chains).Error
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	chainMap := make(map[uint]model.Chain, len(chains))
	for _, chain := range chains {
		chainMap[chain.ID] = chain
	}
	var disks []model.Disk
	err = db.Model(new(model.Disk)).Find(&disks).Error
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	diskMap := make(map[uint]model.Disk)
	for _, disk := range disks {
		diskMap[disk.ID] = disk
	}
	var storServices []model.StorService
	err = db.Model(new(model.StorService)).Find(&storServices).Error
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	storServiceMap := make(map[uint]model.StorService)
	for _, storService := range storServices {
		storServiceMap[storService.NodeID] = storService
	}
	var targets []model.Target
	err = db.Model(new(model.Target)).Find(&targets).Error
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	fsTargets := make([]fsTarget, len(targets))
	ret := chainDistribution{}
	for i, target := range targets {
		chain := chainMap[target.ChainID]
		storService := storServiceMap[target.NodeID]
		disk := diskMap[target.DiskID]
		fsChainID, err := strconv.ParseInt(chain.Name, 10, 64)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		_, ok := ret[fsChainID]
		if !ok {
			ret[fsChainID] = *utils.NewSet[diskOnNode]()
		}
		ret[fsChainID].Add(diskOnNode{
			nodeID:    storService.FsNodeID,
			diskIndex: disk.Index,
		})
		targetIDI, err := strconv.ParseInt(target.Name, 10, 64)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		chainIDI, err := strconv.ParseInt(chain.Name, 10, 64)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		fsTargets[i] = fsTarget{
			ID:        targetIDI,
			NodeID:    storService.FsNodeID,
			DiskIndex: disk.Index,
			ChainID:   chainIDI,
		}
	}

	return ret, fsTargets, nil
}

func (s *prepareChangePlanStep) parseDataPlacementModel(ctx context.Context, modelPath string) (
	chainDistribution, []fsTarget, error) {

	storNum := len(s.Runtime.Cfg.Services.Storage.Nodes)
	targetNumPerDisk := s.Runtime.Cfg.Services.Storage.TargetNumPerDisk
	diskNumPerNode := s.Runtime.Cfg.Services.Storage.DiskNumPerNode
	targetIDPrefix := s.Runtime.Cfg.Services.Storage.TargetIDPrefix
	chainIDPrefix := s.Runtime.Cfg.Services.Storage.ChainIDPrefix
	_, err := s.Em.Docker.Exec(ctx,
		s.Runtime.Cfg.Services.Mgmtd.ContainerName,
		"python3", "/opt/3fs/data_placement/src/setup/gen_chain_table.py",
		"--chain_table_type", "CR",
		"--node_id_begin", "10001",
		"--node_id_end", strconv.Itoa(10000+storNum),
		"--num_disks_per_node", strconv.Itoa(diskNumPerNode),
		"--num_targets_per_disk", strconv.Itoa(targetNumPerDisk),
		"--target_id_prefix", strconv.FormatInt(targetIDPrefix, 10),
		"--chain_id_prefix", strconv.FormatInt(chainIDPrefix, 10),
		"--incidence_matrix_path", modelPath,
	)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "run gen_chain_table.py")
	}

	// read file
	createTargetCmd, err := s.Em.Docker.Exec(ctx,
		s.Runtime.Cfg.Services.Mgmtd.ContainerName,
		"cat", "output/create_target_cmd.txt",
	)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	// sample output:
	// create-target --node-id 10001 --disk-index 0 --target-id 101000100101 --chain-id 900100001  --use-new-chunk-engine
	re := regexp.MustCompile(
		`create-target --node-id (\d+) --disk-index (\d+) --target-id (\d+) --chain-id (\d+)  --use-new-chunk-engine`)
	targets := []fsTarget{}
	for _, line := range strings.Split(createTargetCmd, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		matches := re.FindStringSubmatch(line)
		if len(matches) < 5 {
			return nil, nil, errors.Errorf("failed to parse create-target command: %s", line)
		}
		id, err := strconv.ParseInt(matches[3], 10, 64)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		nodeID, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		chainID, err := strconv.ParseInt(matches[4], 10, 64)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		diskIndex, err := strconv.ParseInt(matches[2], 10, 64)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}

		targets = append(targets, fsTarget{
			ID:        id,
			NodeID:    nodeID,
			DiskIndex: int(diskIndex),
			ChainID:   chainID,
		})
	}
	targetMap := make(map[int64]fsTarget)
	for _, target := range targets {
		targetMap[target.ID] = target
	}

	generatedChainsCmd, err := s.Em.Docker.Exec(ctx,
		s.Runtime.Cfg.Services.Mgmtd.ContainerName,
		"cat", "output/generated_chains.csv",
	)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	chains := []fsChain{}
	for _, line := range strings.Split(generatedChainsCmd, "\n")[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// sample output:
		// ChainId,TargetId,TargetId
		// 900100001,101000100101,101000200101
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			return nil, nil, errors.Errorf("failed to parse generated_chains: %s", line)
		}
		targets := []targetID{}
		for _, part := range parts[1:] {
			targetID, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return nil, nil, errors.Trace(err)
			}
			targets = append(targets, targetID)
		}
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		chains = append(chains, fsChain{
			id:        id,
			targetIDs: targets,
		})
	}

	// chainID -> node of target set
	nodeDistribution := make(chainDistribution)
	for _, chain := range chains {
		targets := chain.targetIDs
		nodes := make(utils.Set[diskOnNode])
		for _, targetID := range targets {
			target, ok := targetMap[targetID]
			if !ok {
				return nil, nil, errors.Errorf("target not found: %d", targetID)
			}
			nodes.Add(diskOnNode{target.NodeID, target.DiskIndex})
		}
		nodeDistribution[chain.id] = nodes
	}

	return nodeDistribution, targets, nil
}

func (s *prepareChangePlanStep) execDataPlacementScript(
	ctx context.Context, nodeNum int, incidenceMatrixPath string) (string, error) {

	replicaFactor := s.Runtime.Cfg.Services.Storage.ReplicationFactor
	targetNumPerDisk := s.Runtime.Cfg.Services.Storage.TargetNumPerDisk
	args := []string{
		"python3", "/opt/3fs/data_placement/src/model/data_placement.py",
		"-ql", "-relax", "-type", "CR", "--num_nodes", strconv.Itoa(nodeNum),
		"--replication_factor", strconv.Itoa(replicaFactor),
		"--min_targets_per_disk", strconv.Itoa(targetNumPerDisk),
	}
	if len(incidenceMatrixPath) > 0 {
		args = append(args, "--existing_incidence_matrix", incidenceMatrixPath)
	}
	args = append(args, "2>&1")

	output, err := s.Em.Docker.Exec(ctx,
		s.Runtime.Cfg.Services.Mgmtd.ContainerName,
		"bash", "-c", fmt.Sprintf("%q", strings.Join(args, " ")),
	)
	if err != nil {
		return "", errors.Trace(err)
	}

	// sample output:
	// 2025-05-21 06:17:43.921 | SUCCESS  | __main__:run:148 - saved solution to:
	//  output/DataPlacementModel-v_2-b_32-r_32-k_2-λ_32-lb_1-ub_0
	re := regexp.MustCompile(`saved solution to: (.*)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return "", errors.Errorf("failed to extract solution file path from output: %s", output)
	}
	modelPath := filepath.Join(matches[1], "incidence_matrix.pickle")
	return modelPath, nil
}

var chainTargetRe = regexp.MustCompile(`(\d+)\((\S+)\)`)

type runChangePlanStep struct {
	task.BaseStep
}

func (s *runChangePlanStep) Execute(ctx context.Context) error {
	changePlan := s.Runtime.LoadChangePlan()
	steps := s.Runtime.LoadChangePlanSteps()

	s.Logger.Infof("Start %s", changePlan)

	db := s.Runtime.LoadDB()
	if changePlan.StartAt == nil {
		changePlan.StartAt = common.Pointer(time.Now())
		if err := db.Model(changePlan).Update("start_at", changePlan.StartAt).Error; err != nil {
			return errors.Trace(err)
		}
	}

	for _, step := range steps {
		if step.StartAt == nil {
			s.Logger.Infof("Running %s with %s", step, step.OperationData)
			step.StartAt = common.Pointer(time.Now())
			if err := db.Model(step).Update("start_at", step.StartAt).Error; err != nil {
				return errors.Trace(err)
			}
		} else if step.FinishAt == nil {
			s.Logger.Infof("Continue %s with %s", step, step.OperationData)
		}
		if step.FinishAt != nil {
			s.Logger.Infof("%s already finished", step)
			continue
		}

		err := func() error {
			switch step.OperationType {
			case model.ChangePlanStepOpType.CreateStorService:
				// create storage service finished in previous task
				return nil
			case model.ChangePlanStepOpType.OfflineTarget:
				return s.checkOfflineTarget(ctx, step)
			case model.ChangePlanStepOpType.RemoveTargetFromChain:
				return s.checkRemoveTargetFromChain(ctx, step)
			case model.ChangePlanStepOpType.RemoveTarget:
				return s.checkRemoveTarget(ctx, step)
			case model.ChangePlanStepOpType.CreateTarget:
				return s.checkCreateTarget(ctx, step)
			case model.ChangePlanStepOpType.AddTargetToChain:
				return s.checkAddTargetToChain(ctx, step)
			case model.ChangePlanStepOpType.UploadChains:
				return s.checkUploadChains(ctx, step)
			case model.ChangePlanStepOpType.UploadChainTable:
				return s.checkUploadChainTable(ctx, step)
			case model.ChangePlanStepOpType.SyncChainAndTargetModel:
				return s.checkSyncChainAndTargets(ctx)
			default:
				return errors.Errorf("invalid operation type %s", step.OperationType)
			}
		}()
		if err != nil {
			return errors.Trace(err)
		}

		step.FinishAt = common.Pointer(time.Now())
		if err = db.Model(step).Update("finish_at", step.FinishAt).Error; err != nil {
			return errors.Trace(err)
		}

		s.Logger.Infof("%s finished", step)
	}

	changePlan.FinishAt = common.Pointer(time.Now())
	if err := db.Model(changePlan).Update("finish_at", changePlan.FinishAt).Error; err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (s *runChangePlanStep) runAdminCli(ctx context.Context, cmd string) (string, error) {
	return s.RunAdminCli(ctx, s.Runtime.Services.Mgmtd.ContainerName, cmd)
}

func (s *runChangePlanStep) loadTargetsInfo(
	ctx context.Context, containOrphan ...bool) (map[targetID]fsTarget, error) {

	targets := make(map[targetID]fsTarget)
	out, err := s.runAdminCli(ctx, "list-targets")
	if err != nil {
		return nil, errors.Trace(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(out))
	scanner.Scan()
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 8 {
			return nil, errors.Errorf("invalid list-targets output line: %s", line)
		}
		targetID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, errors.Annotatef(err, "invalid target id of list-targets output line: %s", line)
		}
		nodeID, err := strconv.ParseInt(parts[5], 10, 64)
		if err != nil {
			return nil, errors.Annotatef(err, "invalid node id of list-targets output line: %s", line)
		}
		diskIdx, err := strconv.ParseInt(parts[6], 10, 64)
		if err != nil {
			return nil, errors.Annotatef(err, "invalid disk index of list-targets output line: %s", line)
		}
		chainID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, errors.Annotatef(err, "invalid chain id of list-targets output line: %s", line)
		}
		targets[targetID] = fsTarget{
			ID:        targetID,
			NodeID:    nodeID,
			DiskIndex: diskIndex(diskIdx),
			ChainID:   chainID,
			status:    fmt.Sprintf("%s-%s", parts[3], parts[4]),
		}
	}

	if len(containOrphan) == 0 || !containOrphan[0] {
		return targets, nil
	}

	// Output of "list-targets --orphan":
	// TargetId      LocalState  NodeId  DiskIndex  UsedSize
	// 101000200227  OFFLINE     10002   1          0
	out, err = s.runAdminCli(ctx, "list-targets --orphan")
	if err != nil {
		return nil, errors.Annotate(err, "list orphan targets")
	}

	scanner = bufio.NewScanner(strings.NewReader(out))
	scanner.Scan()
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 5 {
			return nil, errors.Errorf("invalid \"list-target --orphan\" output line: %s", line)
		}
		targetID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, errors.Annotatef(err, "invalid target id of \"list target --orphan\" output line: %s", line)
		}
		nodeID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return nil, errors.Annotatef(err, "invalid node id of \"list target --orphan\" output line: %s", line)
		}
		diskIdx, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return nil, errors.Annotatef(err, "invalid disk index of \"list target --orphan\" output line: %s", line)
		}
		targets[targetID] = fsTarget{
			ID:        targetID,
			NodeID:    nodeID,
			DiskIndex: diskIndex(diskIdx),
			status:    parts[1],
		}
	}

	return targets, nil
}

func (s *runChangePlanStep) loadChainsInfo(ctx context.Context) (
	map[string]string, map[string]*utils.Set[string], error) {

	out, err := s.runAdminCli(ctx, "list-chains")
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	targetChainMap := map[string]string{}
	chainTargetStatus := map[string]*utils.Set[string]{}
	scanner := bufio.NewScanner(strings.NewReader(out))
	scanner.Scan()
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 5 {
			return nil, nil, errors.Errorf("invalid list-chains output line: %s", line)
		}

		//nolint:lll
		/* output of list-chains:
		   ChainId    ReferencedBy  ChainVersion  Status   PreferredOrder  Target                          Target
		   900100001  1             1             SERVING  []              101000100101(SERVING-UPTODATE)  101000200101(SERVING-UPTODATE)
		   900100002  1             1             SERVING  []              101000100102(SERVING-UPTODATE)  101000200102(SERVING-UPTODATE) */
		chainID := parts[0]
		for _, targetInfo := range parts[5:] {
			matches := chainTargetRe.FindStringSubmatch(targetInfo)
			if len(matches) == 0 {
				return nil, nil, errors.Errorf("invalid target info of list-chains output line: %s", line)
			}
			targetChainMap[matches[1]] = chainID
			set, ok := chainTargetStatus[chainID]
			if !ok {
				set = utils.NewSet[string]()
				chainTargetStatus[chainID] = set
			}
			set.Add(matches[2])
		}
	}

	return targetChainMap, chainTargetStatus, nil
}

func (s *runChangePlanStep) waitTargetChainTargetState(ctx context.Context, targetID targetID,
	pubState, localState string) error {

	timer := time.NewTimer(s.Runtime.Cfg.Services.Mgmtd.WaitTargetOnlineTimeout)
	defer timer.Stop()
	targetState := fmt.Sprintf("%s-%s", pubState, localState)
loop:
	for {
		select {
		case <-timer.C:
			break loop
		default:
			targetChainMap, chainTargetStatus, err := s.loadChainsInfo(ctx)
			if err != nil {
				return errors.Annotate(err, "load chains info")
			}
			targetIDStr := strconv.FormatInt(targetID, 10)
			targetChainID, ok := targetChainMap[targetIDStr]
			if !ok {
				return errors.Errorf("chain info of target %d not found", targetID)
			}
			if chainTargetStatus[targetChainID].Contains(targetState) {
				return nil
			}
		}
	}

	return errors.Errorf("timeout to wait other target of target %d's chain to %s", targetID, targetState)
}

func (s *runChangePlanStep) checkOfflineTarget(ctx context.Context, step *model.ChangePlanStep) error {
	offlineTarget := offlineTargetOpData{}
	if err := json.Unmarshal([]byte(step.OperationData), &offlineTarget); err != nil {
		return errors.Trace(err)
	}

	targets, err := s.loadTargetsInfo(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	if targets[offlineTarget.TargetID].status == "OFFLINE-OFFLINE" {
		s.Logger.Infof("Target %d already offline, skip it.", offlineTarget.TargetID)
		return nil
	}

	// a chain should has at least one serving target to serve, so we should ensure that at least another
	//  target is up-to-date in chain before offline target
	if err := s.waitTargetChainTargetState(ctx, offlineTarget.TargetID, "SERVING", "UPTODATE"); err != nil {
		return errors.Trace(err)
	}

	cmd := fmt.Sprintf("offline-target --target-id %d", offlineTarget.TargetID)
	_, err = s.runAdminCli(ctx, cmd)
	if err != nil {
		return errors.Trace(err)
	}

	if s.waitTargetState(ctx, offlineTarget.TargetID, "OFFLINE", "OFFLINE") != nil {
		return errors.Trace(err)
	}

	return nil
}

func (s *runChangePlanStep) waitChainHasTarget(
	ctx context.Context, targetID targetID, chainID chainID, has bool) error {

	targetIDStr := strconv.FormatInt(targetID, 10)
	chainIDStr := strconv.FormatInt(chainID, 10)
	hasMsg := "has"
	if !has {
		hasMsg = "has no"
	}
	timer := time.NewTimer(s.Runtime.Cfg.Services.Mgmtd.WaitTargetOnlineTimeout)
	defer timer.Stop()
loop:
	for {
		select {
		case <-timer.C:
			break loop
		default:
			targetChainMap, _, err := s.loadChainsInfo(ctx)
			if err != nil {
				return errors.Annotate(err, "load chains info")
			}
			curChainID, ok := targetChainMap[targetIDStr]
			val := ok && chainIDStr == curChainID
			if !has {
				val = !val
			}
			if val {
				return nil
			}
			time.Sleep(time.Second * 5)
		}
	}

	return errors.Errorf("timeout to wait chain %d %s target %d", targetID, hasMsg, chainID)
}

func (s *runChangePlanStep) checkRemoveTargetFromChain(ctx context.Context, step *model.ChangePlanStep) error {
	removeTargetFromChain := removeTargetFromChainOpData{}
	if err := json.Unmarshal([]byte(step.OperationData), &removeTargetFromChain); err != nil {
		return errors.Trace(err)
	}

	targetChainMap, _, err := s.loadChainsInfo(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	if _, ok := targetChainMap[strconv.FormatInt(removeTargetFromChain.TargetID, 10)]; !ok {
		s.Logger.Infof("Target %d already removed from %d, skip it.",
			removeTargetFromChain.TargetID, removeTargetFromChain.ChainID)
		return nil
	}

	chainID := removeTargetFromChain.ChainID
	targetID := removeTargetFromChain.TargetID
	cmd := fmt.Sprintf("update-chain --mode remove %d %d", chainID, targetID)
	_, err = s.runAdminCli(ctx, cmd)
	if err != nil {
		return errors.Trace(err)
	}

	if err = s.waitChainHasTarget(ctx, chainID, targetID, false); err != nil {
		return errors.Trace(err)
	}

	// when we remove target from chain, storage service targe state update has a delay,
	// so we need to wait for a while to ensure the target state is updated
	time.Sleep(10 * time.Second)

	return nil
}

func (s *runChangePlanStep) checkRemoveTarget(ctx context.Context, step *model.ChangePlanStep) error {
	removeTarget := removeTargetOpData{}
	if err := json.Unmarshal([]byte(step.OperationData), &removeTarget); err != nil {
		return errors.Trace(err)
	}

	// NOTE: list-targets only print targets in chains, when check for target exists we should list
	// orphan targets also
	targets, err := s.loadTargetsInfo(ctx, true)
	if err != nil {
		return errors.Trace(err)
	}
	if _, ok := targets[removeTarget.TargetID]; !ok {
		s.Logger.Infof("Target %d already removed, skip it.", removeTarget.TargetID)
		return nil
	}

	cmd := fmt.Sprintf("remove-target --node-id %d --target-id %d",
		removeTarget.NodeID, removeTarget.TargetID)
	_, err = s.runAdminCli(ctx, cmd)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (s *runChangePlanStep) checkCreateTarget(ctx context.Context, step *model.ChangePlanStep) error {
	createTarget := createTargetOpData{}
	if err := json.Unmarshal([]byte(step.OperationData), &createTarget); err != nil {
		return errors.Trace(err)
	}

	targets, err := s.loadTargetsInfo(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	target := targets[createTarget.TargetID]
	if target.ChainID == createTarget.ChainID && target.NodeID == createTarget.NodeID &&
		target.DiskIndex == createTarget.DiskIndex && target.ID == createTarget.TargetID {

		s.Logger.Infof("Target %v already exits, skip create it.", target)
		return nil
	}

	cmd := fmt.Sprintf("create-target --node-id %d --target-id %d --disk-index %d --chain-id %d"+
		" --use-new-chunk-engine",
		createTarget.NodeID, createTarget.TargetID, createTarget.DiskIndex, createTarget.ChainID)
	_, err = s.runAdminCli(ctx, cmd)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (s *runChangePlanStep) waitTargetState(ctx context.Context, targetID targetID, pubState, localState string) error {
	targetIDStr := strconv.FormatInt(targetID, 10)
	timer := time.NewTimer(s.Runtime.Cfg.Services.Mgmtd.WaitTargetOnlineTimeout)
	defer timer.Stop()
loop:
	for {
		select {
		case <-timer.C:
			break loop
		default:
			out, err := s.runAdminCli(ctx, "list-targets")
			if err != nil {
				return errors.Trace(err)
			}
			scanner := bufio.NewScanner(strings.NewReader(out))
			scanner.Scan()
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.Fields(line)
				if len(parts) < 8 {
					return errors.Errorf("invalid list-targets output line: %s", line)
				}
				if targetIDStr != parts[0] {
					continue
				}
				if parts[4] == localState && parts[3] == pubState {
					return nil
				}
			}
			time.Sleep(time.Second)
		}
	}

	return errors.Errorf("timeout to wait target %d SERVING-UPTODATE", targetID)
}

func (s *runChangePlanStep) checkAddTargetToChain(ctx context.Context, step *model.ChangePlanStep) error {
	addTargetToChain := addTargetToChainOpData{}
	if err := json.Unmarshal([]byte(step.OperationData), &addTargetToChain); err != nil {
		return errors.Trace(err)
	}

	targetChainMap, _, err := s.loadChainsInfo(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	doAdd := true
	chainIDStr, ok := targetChainMap[strconv.FormatInt(addTargetToChain.TargetID, 10)]
	if ok && chainIDStr == strconv.FormatInt(addTargetToChain.ChainID, 10) {
		s.Logger.Infof("Target %d already added to chain %d, skip add it.",
			addTargetToChain.TargetID, addTargetToChain.ChainID)
		doAdd = false
	}

	if doAdd {
		cmd := fmt.Sprintf("update-chain --mode add %d %d",
			addTargetToChain.ChainID, addTargetToChain.TargetID)
		_, err = s.runAdminCli(ctx, cmd)
		if err != nil {
			return errors.Trace(err)
		}
	}

	if err := s.waitTargetState(ctx, addTargetToChain.TargetID, "SERVING", "UPTODATE"); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (s *runChangePlanStep) checkUploadChains(ctx context.Context, step *model.ChangePlanStep) error {
	uploadChains := uploadChainsOpData{}
	if err := json.Unmarshal([]byte(step.OperationData), &uploadChains); err != nil {
		return errors.Trace(err)
	}

	chainTargets := map[string][]string{}
	for targetID, chainID := range uploadChains.TargetChainMapping {
		chainIDStr := strconv.FormatInt(chainID, 10)
		targetIDStr := strconv.FormatInt(targetID, 10)
		chainTargets[chainIDStr] = append(chainTargets[chainIDStr], targetIDStr)
	}
	targetNum := 0
	for _, targets := range chainTargets {
		targetNum = len(targets)
	}

	localTmpFile, err := s.Runtime.LocalEm.FS.MkTempFile(ctx, os.TempDir())
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err = s.Runtime.LocalEm.FS.RemoveAll(ctx, localTmpFile); err != nil {
			s.Logger.Warnf("Failed to delete tmp file %s:%s", localTmpFile, err)
		}
	}()
	sb := bytes.NewBufferString("")
	sb.WriteString("ChainId")
	for range targetNum {
		sb.WriteString(",TargetId")
	}
	sb.WriteString("\n")
	for chainID, targets := range chainTargets {
		sb.WriteString(fmt.Sprintf("%s\n", strings.Join(append([]string{chainID}, targets...), ",")))
	}

	err = s.Runtime.LocalEm.FS.WriteFile(localTmpFile, sb.Bytes(), 0644)
	if err != nil {
		return errors.Trace(err)
	}

	remoteTmpFile, err := s.Em.FS.MkTempFile(ctx, os.TempDir())
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err = s.Em.FS.RemoveAll(ctx, remoteTmpFile); err != nil {
			s.Logger.Warnf("Failed to delete tmp file %s:%s", remoteTmpFile, err)
		}
	}()
	if err = s.Em.Runner.Scp(ctx, localTmpFile, remoteTmpFile); err != nil {
		return errors.Trace(err)
	}
	if err = s.Em.Docker.Cp(ctx, remoteTmpFile, s.Runtime.Cfg.Services.Mgmtd.ContainerName,
		uploadChains.ChainsFile); err != nil {

		return errors.Trace(err)
	}

	cmd := fmt.Sprintf("upload-chains %s", uploadChains.ChainsFile)
	_, err = s.runAdminCli(ctx, cmd)
	if err != nil {
		return errors.Trace(err)
	}

	err = steps.WaitUtilWithTimeout(ctx, "wait all chains serving", func() (bool, error) {
		_, chainTargetStatus, err := s.loadChainsInfo(ctx)
		if err != nil {
			return false, errors.Trace(err)
		}
		for _, stateSet := range chainTargetStatus {
			for _, state := range stateSet.ToSlice() {
				if state != "SERVING-UPTODATE" {
					return false, nil
				}
			}
		}

		return true, nil
	}, s.Runtime.Cfg.Services.Storage.WaitChainsServingTimeout, time.Second*5)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (s *runChangePlanStep) checkUploadChainTable(ctx context.Context, step *model.ChangePlanStep) error {
	uploadChainTable := uploadChainTableOpData{}
	if err := json.Unmarshal([]byte(step.OperationData), &uploadChainTable); err != nil {
		return errors.Trace(err)
	}

	cmd := fmt.Sprintf("upload-chain-table %d %s",
		uploadChainTable.ChainTableID, uploadChainTable.ChainTableFile)
	_, err := s.runAdminCli(ctx, cmd)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (s *runChangePlanStep) checkSyncChainAndTargets(ctx context.Context) error {
	var chains []model.Chain
	var targets []model.Target
	db := s.Runtime.LoadDB()
	err := db.Model(new(model.Chain)).Find(&chains).Error
	if err != nil {
		return errors.Trace(err)
	}
	err = db.Model(new(model.Target)).Find(&targets).Error
	if err != nil {
		return errors.Trace(err)
	}
	chainMap := make(map[int64]model.Chain, len(chains))
	for _, chain := range chains {
		chainID, err := strconv.ParseInt(chain.Name, 10, 64)
		if err != nil {
			return errors.Errorf("invalid chain name %s", chain.Name)
		}
		chainMap[chainID] = chain
	}
	targetMap := make(map[int64]model.Target, len(targets))
	for _, target := range targets {
		targetID, err := strconv.ParseInt(target.Name, 10, 64)
		if err != nil {
			return errors.Errorf("invalid target name %s", target.Name)
		}
		targetMap[targetID] = target
	}
	var storServices []model.StorService
	err = db.Model(new(model.StorService)).Find(&storServices).Error
	if err != nil {
		return errors.Trace(err)
	}
	storServiceMap := make(map[int64]model.StorService, len(storServices))
	for _, storService := range storServices {
		storServiceMap[storService.FsNodeID] = storService
	}
	var disks []model.Disk
	err = db.Model(new(model.Disk)).Find(&disks).Error
	if err != nil {
		return errors.Trace(err)
	}
	diskKeyFunc := func(nodeID uint, diskIndex int) string {
		return fmt.Sprintf("%d-%d", nodeID, diskIndex)
	}
	diskMap := make(map[string]model.Disk, len(disks))
	for _, disk := range disks {
		diskMap[diskKeyFunc(disk.NodeID, disk.Index)] = disk
	}

	fsTargets, fsChains, err := s.getFsChainsAndTargets(ctx)
	if err != nil {
		return errors.Trace(err)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		for chainIDStr, fsChain := range fsChains {
			_, ok := chainMap[fsChain.id]
			if ok {
				continue
			}
			var chain model.Chain
			chain.Name = chainIDStr
			err := tx.Model(&chain).Create(&chain).Error
			if err != nil {
				return errors.Trace(err)
			}
			chainMap[fsChain.id] = chain
		}

		for _, fsTarget := range fsTargets {
			target, ok := targetMap[fsTarget.ID]
			if ok {
				targetNewChain := chainMap[fsTarget.ChainID]
				if target.ChainID != targetNewChain.ID {
					target.ChainID = targetNewChain.ID
					if err = tx.Model(&target).Update("chain_id", target.ChainID).Error; err != nil {
						return errors.Trace(err)
					}
				}
				continue
			}
			chain, ok := chainMap[fsTarget.ChainID]
			if !ok {
				return errors.Errorf("chain %d for target %d not found", fsTarget.ChainID, fsTarget.ID)
			}
			storService, ok := storServiceMap[fsTarget.NodeID]
			if !ok {
				return errors.Errorf("stor service %d for target %d not found", fsTarget.NodeID, fsTarget.ID)
			}
			disk, ok := diskMap[diskKeyFunc(storService.NodeID, fsTarget.DiskIndex)]
			if !ok {
				return errors.Errorf("disk for target %d not found", fsTarget.ID)
			}
			target = model.Target{
				Name:    strconv.FormatInt(fsTarget.ID, 10),
				DiskID:  disk.ID,
				NodeID:  storService.NodeID,
				ChainID: chain.ID,
			}
			target.Name = strconv.FormatInt(fsTarget.ID, 10)
			err := tx.Model(&target).Create(&target).Error
			if err != nil {
				return errors.Trace(err)
			}
		}

		return nil
	})
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (s *runChangePlanStep) getFsChainsAndTargets(ctx context.Context) (
	map[string]*fsTarget, map[string]*fsChain, error) {

	out, err := s.runAdminCli(ctx, "list-targets")
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(out))
	scanner.Scan()
	targetMap := make(map[string]*fsTarget)
	chainMap := make(map[string]*fsChain)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 8 {
			return nil, nil, errors.Errorf("invalid list-targets output line: %s", line)
		}
		targetIDStr := parts[0]
		targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		chainIDStr := parts[1]
		chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		nodeIDStr := parts[5]
		nodeID, err := strconv.ParseInt(nodeIDStr, 10, 64)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		diskIDStr := parts[6]
		diskID, err := strconv.ParseInt(diskIDStr, 10, 64)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		targetMap[targetIDStr] = &fsTarget{
			ID:        targetID,
			NodeID:    nodeID,
			DiskIndex: int(diskID),
			ChainID:   chainID,
		}
		chain, ok := chainMap[chainIDStr]
		if !ok {
			chain = &fsChain{id: chainID}
			chainMap[chainIDStr] = chain
		}
		chain.targetIDs = append(chain.targetIDs, targetID)
	}

	return targetMap, chainMap, nil
}
