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

package monitor

import (
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/task"
	"github.com/open3fs/m3fs/pkg/task/steps"
)

// CreateMonitorTask is a task for creating a 3fs monitor.
type CreateMonitorTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *CreateMonitorTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("CreateMonitorTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Monitor.Nodes))
	for i, node := range r.Cfg.Services.Monitor.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: func() task.Step { return new(genMonitorConfigStep) },
		},
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: func() task.Step { return new(runContainerStep) },
		},
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: steps.NewCleanupLocalStepFunc(task.RuntimeMonitorTmpDirKey),
		},
	})
}

// DeleteMonitorTask is a task for deleting a 3fs monitor.
type DeleteMonitorTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *DeleteMonitorTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("DeleteMonitorTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Monitor.Nodes))
	for i, node := range r.Cfg.Services.Monitor.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(rmContainerStep) },
		},
	})
}
