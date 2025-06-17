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

package mgmtd

import (
	"fmt"
	"path"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
	"github.com/open3fs/m3fs/pkg/task/steps"
)

// ServiceName is the name of the mgmtd service.
const ServiceName = "mgmtd_main"

func getServiceWorkDir(workDir string) string {
	return path.Join(workDir, "mgmtd")
}

// CreateMgmtdServiceTask is a task for creating 3fs mgmtd services.
type CreateMgmtdServiceTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *CreateMgmtdServiceTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("CreateMgmtdServiceTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Mgmtd.Nodes))
	for i, node := range r.Cfg.Services.Mgmtd.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: steps.NewGen3FSNodeIDStepFunc(ServiceName, 1, r.Cfg.Services.Mgmtd.Nodes),
		},
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: func() task.Step { return new(genAdminCliConfigStep) },
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewPrepare3FSConfigStepFunc(&steps.Prepare3FSConfigStepSetup{
				Service:              ServiceName,
				ServiceWorkDir:       getServiceWorkDir(r.WorkDir),
				MainAppTomlTmpl:      MgmtdMainAppTomlTmpl,
				MainLauncherTomlTmpl: MgmtdMainLauncherTomlTmpl,
				MainTomlTmpl:         MgmtdMainTomlTmpl,
				RDMAListenPort:       r.Services.Mgmtd.RDMAListenPort,
				TCPListenPort:        r.Services.Mgmtd.TCPListenPort,
			}),
		},
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: func() task.Step { return new(initClusterStep) },
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewRun3FSContainerStepFunc(
				&steps.Run3FSContainerStepSetup{
					ImgName:        config.ImageName3FS,
					ContainerName:  r.Services.Mgmtd.ContainerName,
					Service:        ServiceName,
					WorkDir:        getServiceWorkDir(r.WorkDir),
					UseRdmaNetwork: true,
					ModelObjFunc: func(s *task.BaseStep) any {
						fsNodeID, _ := s.Runtime.LoadInt(
							steps.GetNodeIDKey(ServiceName, s.Node.Name))
						return &model.MgmtService{
							Name:     r.Services.Mgmtd.ContainerName,
							NodeID:   s.GetNodeModelID(),
							FsNodeID: fmt.Sprintf("%d", fsNodeID),
						}
					},
				}),
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep:  func() task.Step { return new(genAdminCliShellStep) },
		},
	})
}

// DeleteMgmtdServiceTask is a task for deleting a mgmtd services.
type DeleteMgmtdServiceTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *DeleteMgmtdServiceTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("DeleteMgmtdServiceTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Mgmtd.Nodes))
	for i, node := range r.Cfg.Services.Mgmtd.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewRm3FSContainerStepFunc(
				r.Services.Mgmtd.ContainerName,
				ServiceName,
				getServiceWorkDir(r.WorkDir)),
		},
	})
}

// InitUserAndChainTask is a task for initializing user and chain.
type InitUserAndChainTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *InitUserAndChainTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("InitUserAndChainTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Mgmtd.Nodes))
	for i, node := range r.Cfg.Services.Mgmtd.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: func() task.Step { return new(initUserAndChainStep) },
		},
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: func() task.Step { return new(createChainAndTargetModelStep) },
		},
	})
}
