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
	"errors"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/bitly/go-simplejson"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/pg/model"
	ttask "github.com/open3fs/m3fs/tests/task"
)

func TestPrepareChangePlanStepSuite(t *testing.T) {
	suiteRun(t, &prepareChangePlanStepSuite{})
}

type prepareChangePlanStepSuite struct {
	ttask.StepSuite

	step         *prepareChangePlanStep
	node1        model.Node
	node2        model.Node
	newNode      model.Node
	node1Disk    model.Disk
	node2Disk    model.Disk
	node1Stor    model.StorService
	node2Stor    model.StorService
	node1Target1 model.Target
	node1Target2 model.Target
	node2Target1 model.Target
	node2Target2 model.Target
	chain1       model.Chain
	chain2       model.Chain
}

func (s *prepareChangePlanStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.node1 = model.Node{Name: "node1", Host: "host1"}
	s.node2 = model.Node{Name: "node2", Host: "host2"}
	s.newNode = model.Node{Name: "node3", Host: "host3"}
	s.Cfg.Nodes = []config.Node{
		{
			Name: s.node1.Name,
			Host: s.node1.Host,
		},
		{
			Name: s.node2.Name,
			Host: s.node2.Host,
		},
		{
			Name: s.newNode.Name,
			Host: s.newNode.Host,
		},
	}
	s.Cfg.Services.Storage.Nodes = []string{s.node1.Name, s.node2.Name, s.newNode.Name}
	s.step = &prepareChangePlanStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
	db := s.NewDB()
	s.NoError(db.Model(new(model.Node)).First(&s.node1, "name = ?", s.node1.Name).Error)
	s.NoError(db.Model(new(model.Node)).First(&s.node2, "name = ?", s.node2.Name).Error)
	s.NoError(db.Model(new(model.Node)).First(&s.newNode, "name = ?", s.newNode.Name).Error)
	s.node1Stor.NodeID = s.node1.ID
	s.node1Stor.FsNodeID = 10001
	s.node2Stor.NodeID = s.node2.ID
	s.node2Stor.FsNodeID = 10002
	s.NoError(db.Model(new(model.StorService)).
		Create([]*model.StorService{&s.node1Stor, &s.node2Stor}).Error)
	s.node1Disk = model.Disk{NodeID: s.node1.ID, StorServiceID: s.node1Stor.ID}
	s.node2Disk = model.Disk{NodeID: s.node2.ID, StorServiceID: s.node2Stor.ID}
	s.NoError(db.Model(new(model.Disk)).Create([]*model.Disk{&s.node1Disk, &s.node2Disk}).Error)

	s.chain1.Name = "900100001"
	s.chain2.Name = "900100002"
	s.NoError(db.Model(new(model.Chain)).Create([]*model.Chain{&s.chain1, &s.chain2}).Error)
	s.node1Target1 = s.initTarget("101000100101", s.chain1, s.node1, s.node1Disk)
	s.node1Target2 = s.initTarget("101000100102", s.chain2, s.node1, s.node1Disk)
	s.node2Target1 = s.initTarget("101000200101", s.chain1, s.node2, s.node2Disk)
	s.node2Target2 = s.initTarget("101000200102", s.chain2, s.node2, s.node2Disk)
}
func (s *prepareChangePlanStepSuite) initTarget(name string, chain model.Chain, node model.Node,
	disk model.Disk) model.Target {

	target := model.Target{
		Name:    name,
		ChainID: chain.ID,
		NodeID:  node.ID,
		DiskID:  disk.ID,
	}
	s.NoError(s.NewDB().Model(new(model.Target)).Create(&target).Error)
	return target
}

func (s *prepareChangePlanStepSuite) mockRunStep() []model.ChangePlanStep {
	container := s.Runtime.Cfg.Services.Mgmtd.ContainerName
	storCfg := s.Runtime.Cfg.Services.Storage
	nodeNum := len(storCfg.Nodes)
	replicationFactor := storCfg.ReplicationFactor
	targetPerNode := storCfg.TargetNumPerDisk

	modelFile := "output/DataPlacementModel-v_2-b_32-r_32-k_2-λ_32-lb_1-ub_0"
	s.MockDocker.On("Exec", container, "bash",
		[]string{
			"-c",
			"python3 /opt/3fs/data_placement/src/model/data_placement.py " +
				"-ql -relax -type CR " +
				"--num_nodes 2 " +
				"--replication_factor " + strconv.Itoa(replicationFactor) + " " +
				"--min_targets_per_disk " + strconv.Itoa(targetPerNode) + " " +
				"2>&1",
		}).Return("saved solution to: "+modelFile, nil).Once()

	model2File := "output/RebalanceTrafficModel-v_3-b_48-r_32-k_2-λ_16-lb_1-ub_0"
	s.MockDocker.On("Exec", container, "bash",
		[]string{"-c",
			"python3 /opt/3fs/data_placement/src/model/data_placement.py " +
				"-ql -relax -type CR " +
				"--num_nodes " + strconv.Itoa(nodeNum) + " " +
				"--replication_factor " + strconv.Itoa(replicationFactor) + " " +
				"--min_targets_per_disk " + strconv.Itoa(targetPerNode) + " " +
				"--existing_incidence_matrix " + filepath.Join(modelFile, "incidence_matrix.pickle") + " " +
				"2>&1",
		}).Return("saved solution to: "+model2File, nil)

	s.MockDocker.On("Exec", container, "python3",
		[]string{"/opt/3fs/data_placement/src/setup/gen_chain_table.py",
			"--chain_table_type", "CR",
			"--node_id_begin", "10001",
			"--node_id_end", strconv.Itoa(10000 + nodeNum),
			"--num_disks_per_node", strconv.Itoa(storCfg.DiskNumPerNode),
			"--num_targets_per_disk", strconv.Itoa(targetPerNode),
			"--target_id_prefix", strconv.Itoa(int(storCfg.TargetIDPrefix)),
			"--chain_id_prefix", strconv.Itoa(int(storCfg.ChainIDPrefix)),
			"--incidence_matrix_path", filepath.Join(model2File, "incidence_matrix.pickle"),
		}).Return("saved solution to: "+model2File, nil)

	//nolint:lll
	createTargetCmd := `create-target --node-id 10001 --disk-index 0 --target-id 101000100101 --chain-id 900100001  --use-new-chunk-engine
create-target --node-id 10002 --disk-index 0 --target-id 101000200101 --chain-id 900100001  --use-new-chunk-engine
create-target --node-id 10001 --disk-index 0 --target-id 101000100102 --chain-id 900100002  --use-new-chunk-engine
create-target --node-id 10003 --disk-index 0 --target-id 101000300101 --chain-id 900100002  --use-new-chunk-engine
create-target --node-id 10002 --disk-index 0 --target-id 101000200102 --chain-id 900100003  --use-new-chunk-engine
create-target --node-id 10003 --disk-index 0 --target-id 101000300102 --chain-id 900100003  --use-new-chunk-engine
	`
	s.MockDocker.On("Exec", container, "cat",
		[]string{"output/create_target_cmd.txt"},
	).Return(createTargetCmd, nil)

	generatedChainsCmd := `ChainId,TargetId,TargetId
900100001,101000100101,101000200101
900100002,101000100102,101000300101
900100003,101000200102,101000300102
`
	s.MockDocker.On("Exec", container, "cat",
		[]string{"output/generated_chains.csv"},
	).Return(generatedChainsCmd, nil)

	return []model.ChangePlanStep{
		{
			OperationType: model.ChangePlanStepOpType.CreateStorService,
		}, {
			OperationType: model.ChangePlanStepOpType.CreateTarget,
			OperationData: string(s.JsonMarshal(createTargetOpData{
				NodeID:    10003,
				DiskIndex: 0,
				TargetID:  101000300101,
				ChainID:   900100002,
			})),
		},
		{
			OperationType: model.ChangePlanStepOpType.AddTargetToChain,
			OperationData: string(s.JsonMarshal(addTargetToChainOpData{
				TargetID: 101000300101,
				ChainID:  900100002,
			})),
		}, {
			OperationType: model.ChangePlanStepOpType.OfflineTarget,
			OperationData: string(s.JsonMarshal(offlineTargetOpData{
				TargetID: 101000200102,
			})),
		}, {
			OperationType: model.ChangePlanStepOpType.RemoveTargetFromChain,
			OperationData: string(s.JsonMarshal(removeTargetFromChainOpData{
				ChainID:  900100002,
				TargetID: 101000200102,
			})),
		}, {
			OperationType: model.ChangePlanStepOpType.RemoveTarget,
			OperationData: string(s.JsonMarshal(removeTargetOpData{
				NodeID:   10002,
				TargetID: 101000200102,
			})),
		}, {
			OperationType: model.ChangePlanStepOpType.CreateTarget,
			OperationData: string(s.JsonMarshal(createTargetOpData{
				NodeID:    10002,
				DiskIndex: 0,
				TargetID:  101000200102,
				ChainID:   900100003,
			})),
		}, {
			OperationType: model.ChangePlanStepOpType.AddTargetToChain,
			OperationData: string(s.JsonMarshal(addTargetToChainOpData{
				TargetID: 101000200102,
				ChainID:  900100003,
			})),
		}, {
			OperationType: model.ChangePlanStepOpType.CreateTarget,
			OperationData: string(s.JsonMarshal(createTargetOpData{
				NodeID:    10003,
				DiskIndex: 0,
				TargetID:  101000300102,
				ChainID:   900100003,
			})),
		}, {
			OperationType: model.ChangePlanStepOpType.AddTargetToChain,
			OperationData: string(s.JsonMarshal(addTargetToChainOpData{
				TargetID: 101000300102,
				ChainID:  900100003,
			})),
		}, {
			OperationType: model.ChangePlanStepOpType.UploadChains,
			OperationData: string(s.JsonMarshal(uploadChainsOpData{
				ChainsFile: "output/generated_chains.csv",
			})),
		}, {
			OperationType: model.ChangePlanStepOpType.UploadChainTable,
			OperationData: string(s.JsonMarshal(uploadChainTableOpData{
				ChainTableID:   1,
				ChainTableFile: "output/generated_chain_table.csv",
			})),
		}, {
			OperationType: model.ChangePlanStepOpType.CreateNewChainAndTargetModel,
			OperationData: string(s.JsonMarshal(createTargetModelOpData{
				Targets: []fsTarget{
					{
						id:        101000100101,
						nodeID:    10001,
						diskIndex: 0,
						chainID:   900100001,
					},
					{
						id:        101000200101,
						nodeID:    10002,
						diskIndex: 0,
						chainID:   900100001,
					},
					{
						id:        101000200102,
						nodeID:    10001,
						diskIndex: 0,
						chainID:   900100002,
					},
					{
						id:        101000300101,
						nodeID:    10003,
						diskIndex: 0,
						chainID:   900100002,
					},
					{
						id:        101000200102,
						nodeID:    10002,
						diskIndex: 0,
						chainID:   900100003,
					},
					{
						id:        101000300102,
						nodeID:    10003,
						diskIndex: 0,
						chainID:   900100003,
					},
				},
				ChainIDs: []chainID{900100001, 900100002, 900100003},
			})),
		},
	}
}

func (s *prepareChangePlanStepSuite) TestRunGenerateStep() {
	stepsExp := s.mockRunStep()

	s.NoError(s.step.Execute(s.Ctx()))

	db := s.NewDB()
	changePlanDB, err := model.GetProcessingChangePlan(db)
	s.NoError(err)
	planData := simplejson.New()
	planData.Set("new_node_ids", []uint{s.newNode.ID})
	changePlanExp := &model.ChangePlan{
		Model: changePlanDB.Model,
		Type:  model.ChangePlanTypeAddStorNodes,
		Data:  s.JsonToString(planData),
	}
	s.Equal(changePlanExp, changePlanDB)
	stepsDB, err := changePlanDB.GetSteps(db)
	s.NoError(err)
	s.Equal(len(stepsExp), len(stepsDB))
	for i, stepExp := range stepsExp {
		stepExp.Model = stepsDB[i].Model
		stepExp.ChangePlanID = changePlanDB.ID
		s.Equal(stepExp, *stepsDB[i])
	}

	s.MockDocker.AssertExpectations(s.T())
}

func (s *prepareChangePlanStepSuite) TestRunGenerateStepFailed() {
	strCfg := s.Runtime.Cfg.Services.Storage
	s.MockDocker.On("Exec", s.Cfg.Services.Mgmtd.ContainerName, "bash",
		[]string{"-c",
			"python3 /opt/3fs/data_placement/src/model/data_placement.py " +
				"-ql -relax -type CR " +
				"--num_nodes 2 " +
				"--replication_factor " + strconv.Itoa(strCfg.ReplicationFactor) + " " +
				"--min_targets_per_disk " + strconv.Itoa(strCfg.TargetNumPerDisk) + " " +
				"2>&1"}, // model path log will be printed to stderr
	).Return(nil, errors.New("failed to generate model")).Once()

	s.Error(s.step.Execute(s.Ctx()), "failed to generate model")

	var count int64
	s.NoError(s.NewDB().Model(new(model.ChangePlan)).Count(&count).Error)
	s.Zero(count)
	s.NoError(s.NewDB().Model(new(model.ChangePlanStep)).Count(&count).Error)
	s.Zero(count)

	s.MockDocker.AssertExpectations(s.T())
}
