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

package meta

import (
	"fmt"
	"path"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
	"github.com/open3fs/m3fs/pkg/task/steps"
)

const (
	// ServiceName is the name of the meta service.
	ServiceName = "meta_main"
	serviceType = "META"
)

func getServiceWorkDir(workDir string) string {
	return path.Join(workDir, "meta")
}

// CreateMetaServiceTask is a task for creating 3fs meta services.
type CreateMetaServiceTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *CreateMetaServiceTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("CreateMetaServiceTask")
	t.BaseTask.Init(r, logger)

	workDir := getServiceWorkDir(r.WorkDir)
	nodes := make([]config.Node, len(r.Cfg.Services.Meta.Nodes))
	for i, node := range r.Cfg.Services.Meta.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: steps.NewGen3FSNodeIDStepFunc(ServiceName, 100, r.Cfg.Services.Meta.Nodes),
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewPrepare3FSConfigStepFunc(&steps.Prepare3FSConfigStepSetup{
				Service:              ServiceName,
				ServiceWorkDir:       workDir,
				MainAppTomlTmpl:      MetaMainAppTomlTmpl,
				MainLauncherTomlTmpl: MetaMainLauncherTomlTmpl,
				MainTomlTmpl:         MetaMainTomlTmpl,
				RDMAListenPort:       r.Services.Meta.RDMAListenPort,
				TCPListenPort:        r.Services.Meta.TCPListenPort,
			}),
		},
		{
			Nodes: []config.Node{nodes[0]},
			NewStep: steps.NewUpload3FSMainConfigStepFunc(
				config.ImageName3FS,
				r.Services.Meta.ContainerName,
				ServiceName,
				workDir,
				serviceType,
			),
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewRun3FSContainerStepFunc(
				&steps.Run3FSContainerStepSetup{
					ImgName:        config.ImageName3FS,
					ContainerName:  r.Services.Meta.ContainerName,
					Service:        ServiceName,
					WorkDir:        workDir,
					UseRdmaNetwork: true,
					ModelObjFunc: func(s *task.BaseStep) any {
						fsNodeID, _ := s.Runtime.LoadInt(
							steps.GetNodeIDKey(ServiceName, s.Node.Name))
						return &model.MetaService{
							Name:     r.Services.Meta.ContainerName,
							NodeID:   s.GetNodeModelID(),
							FsNodeID: fmt.Sprintf("%d", fsNodeID),
						}
					},
				},
			),
		},
	})
}

// DeleteMetaServiceTask is a task for deleting a meta services.
type DeleteMetaServiceTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *DeleteMetaServiceTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("DeleteMetaServiceTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Meta.Nodes))
	for i, node := range r.Cfg.Services.Meta.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewRm3FSContainerStepFunc(
				r.Services.Meta.ContainerName,
				ServiceName,
				getServiceWorkDir(r.WorkDir)),
		},
	})
}
