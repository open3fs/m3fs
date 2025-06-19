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
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
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
	id        targetID
	nodeID    nodeID
	diskIndex diskIndex
	chainID   chainID
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
	ChainsFile string `json:"chains_file"`
}

type uploadChainTableOpData struct {
	ChainTableID   int64  `json:"chain_table_id"`
	ChainTableFile string `json:"chain_table_file"`
}

type createTargetModelOpData struct {
	Targets  []fsTarget `json:"targets"`
	ChainIDs []chainID  `json:"chain_ids"`
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
		node := target.nodeID
		disk := target.diskIndex
		index := target.id
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

	// target index range is [1, 100)
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
	// TODO: check has processing change plan
	if err := s.generateNewChangePlan(ctx); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (s *prepareChangePlanStep) generateNewChangePlan(ctx context.Context) error {
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
	newDistribution, newTargets, err := s.parseDataPlacementModel(ctx, modelPath)
	if err != nil {
		return errors.Annotate(err, "parse new data placement model")
	}
	curDistribution, curTargets, err := s.getCurrentDistribution()
	if err != nil {
		return errors.Trace(err)
	}
	steps, err := s.generateSteps(curDistribution, newDistribution, curTargets, newTargets)
	if err != nil {
		return errors.Annotate(err, "generate change plan steps")
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		changePlan := &model.ChangePlan{
			Type: model.ChangePlanTypeAddStorNodes,
			Data: string(data),
		}
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

	return nil
}

func (s *prepareChangePlanStep) generateSteps(
	currentDistribution chainDistribution,
	newDistribution chainDistribution,
	currentTargets []fsTarget,
	newTargets []fsTarget,
) ([]model.ChangePlanStep, error) {

	storCfg := s.Runtime.Cfg.Services.Storage
	targetPool := newTargetPool(storCfg.TargetIDPrefix, currentTargets)
	currentTargetMap := make(map[string]fsTarget)
	for _, target := range currentTargets {
		// key = chainID:nodeID:diskIndex
		key := fmt.Sprintf("%d:%d:%d", target.chainID, target.nodeID, target.diskIndex)
		currentTargetMap[key] = target
	}

	steps := []model.ChangePlanStep{{OperationType: model.ChangePlanStepOpType.CreateStorService}}
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

			steps = append(steps, model.ChangePlanStep{
				OperationType: model.ChangePlanStepOpType.CreateTarget,
				OperationData: createTargetData,
			}, model.ChangePlanStep{
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
				TargetID: target.id,
			})
			if err != nil {
				return nil, errors.Trace(err)
			}
			removeTargetFromChainData, err := utils.JsonMarshalString(removeTargetFromChainOpData{
				ChainID:  chainID,
				TargetID: target.id,
			})
			if err != nil {
				return nil, errors.Trace(err)
			}
			removeTargetData, err := utils.JsonMarshalString(removeTargetOpData{
				NodeID:   target.nodeID,
				TargetID: target.id,
			})
			if err != nil {
				return nil, errors.Trace(err)
			}
			if err = targetPool.Remove(target.nodeID, target.diskIndex, target.id); err != nil {
				return nil, errors.Trace(err)
			}

			steps = append(steps, model.ChangePlanStep{
				OperationType: model.ChangePlanStepOpType.OfflineTarget,
				OperationData: offlineTargetData,
			}, model.ChangePlanStep{
				OperationType: model.ChangePlanStepOpType.RemoveTargetFromChain,
				OperationData: removeTargetFromChainData,
			}, model.ChangePlanStep{
				OperationType: model.ChangePlanStepOpType.RemoveTarget,
				OperationData: removeTargetData,
			})
		}
	}
	for chainID, diskOnNodeSet := range newDistribution {
		if _, has := currentDistribution[chainID]; has {
			continue
		}
		for diskOnNode := range diskOnNodeSet {
			targetID, err := targetPool.GetAvailableTargetID(diskOnNode.nodeID, diskOnNode.diskIndex)
			if err != nil {
				return nil, errors.Trace(err)
			}
			createTargetData, err := utils.JsonMarshalString(createTargetOpData{
				NodeID:    diskOnNode.nodeID,
				TargetID:  targetID,
				DiskIndex: diskOnNode.diskIndex,
				ChainID:   chainID,
			})
			if err != nil {
				return nil, errors.Trace(err)
			}
			addTargetToChainData, err := utils.JsonMarshalString(addTargetToChainOpData{
				TargetID: targetID,
				ChainID:  chainID,
			})
			if err != nil {
				return nil, errors.Trace(err)
			}

			steps = append(steps, model.ChangePlanStep{
				OperationType: model.ChangePlanStepOpType.CreateTarget,
				OperationData: createTargetData,
			}, model.ChangePlanStep{
				OperationType: model.ChangePlanStepOpType.AddTargetToChain,
				OperationData: addTargetToChainData,
			})
		}
	}

	updateChainsData, err := utils.JsonMarshalString(uploadChainsOpData{
		ChainsFile: "output/generated_chains.csv",
	})
	if err != nil {
		return nil, errors.Trace(err)
	}
	steps = append(steps, model.ChangePlanStep{
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
	steps = append(steps, model.ChangePlanStep{
		OperationType: model.ChangePlanStepOpType.UploadChainTable,
		OperationData: uploadChainTableData,
	})

	createModelData := createTargetModelOpData{
		Targets: newTargets,
	}
	for chainID := range newDistribution {
		createModelData.ChainIDs = append(createModelData.ChainIDs, chainID)
	}
	sort.Slice(createModelData.ChainIDs, func(i, j int) bool {
		return createModelData.ChainIDs[i] < createModelData.ChainIDs[j]
	})
	data, err := utils.JsonMarshalString(createModelData)
	if err != nil {
		return nil, errors.Trace(err)
	}
	steps = append(steps, model.ChangePlanStep{
		OperationType: model.ChangePlanStepOpType.CreateNewChainAndTargetModel,
		OperationData: data,
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
			id:        targetIDI,
			nodeID:    storService.FsNodeID,
			diskIndex: disk.Index,
			chainID:   chainIDI,
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
			id:        id,
			nodeID:    nodeID,
			diskIndex: int(diskIndex),
			chainID:   chainID,
		})
	}
	targetMap := make(map[int64]fsTarget)
	for _, target := range targets {
		targetMap[target.id] = target
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
			nodes.Add(diskOnNode{target.nodeID, target.diskIndex})
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
		"bash", "-c", strings.Join(args, " "),
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
