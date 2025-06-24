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

package network

import (
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/task"
)

// PrepareNetworkTask is a task for preparing network for a new node.
type PrepareNetworkTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *PrepareNetworkTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("PrepareNetworkTask")
	t.BaseTask.Init(r, logger)
	nodes := r.Cfg.Nodes

	steps := []task.StepConfig{}
	switch r.Cfg.NetworkType {
	case config.NetworkTypeRXE:
		rxeSteps := []task.StepConfig{
			{
				Nodes:    nodes,
				Parallel: true,
				NewStep:  func() task.Step { return new(installRdmaPackageStep) },
			},
			{
				Nodes:    nodes,
				Parallel: true,
				NewStep:  func() task.Step { return new(setupRxeStep) },
			},
		}
		steps = append(steps, rxeSteps...)
	case config.NetworkTypeERDMA:
		erdmaSteps := []task.StepConfig{
			{
				Nodes:    nodes,
				Parallel: true,
				NewStep:  func() task.Step { return new(setupErdmaStep) },
			},
		}
		steps = append(steps, erdmaSteps...)
	}
	if r.Cfg.NetworkType != config.NetworkTypeRDMA {
		steps = append(steps, task.StepConfig{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(genIbdev2netdevScriptStep) },
		})
	}
	t.SetSteps(steps)
}

// DeleteNetworkTask is a task for deleting configuration added on setup network.
type DeleteNetworkTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *DeleteNetworkTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("DeleteNetworkTask")
	t.BaseTask.Init(r, logger)
	nodes := r.Cfg.Nodes

	steps := []task.StepConfig{
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep:  func() task.Step { return new(deleteSetupNetworkServiceStep) },
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep:  func() task.Step { return new(deleteIbdev2netdevScriptStep) },
		},
	}

	t.SetSteps(steps)
}
