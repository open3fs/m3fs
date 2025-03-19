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

package fdb

import (
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/task"
)

// CreateFdbClusterTask is a task for creating a new FoundationDB cluster.
type CreateFdbClusterTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *CreateFdbClusterTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("CreateFdbClusterTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Fdb.Nodes))
	for i, node := range r.Cfg.Services.Fdb.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: func() task.Step { return new(genClusterFileContentStep) },
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep:  func() task.Step { return new(runContainerStep) },
		},
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: func() task.Step { return new(initClusterStep) },
		},
	})
}

// DeleteFdbClusterTask is a task for deleting a FoundationDB cluster.
type DeleteFdbClusterTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *DeleteFdbClusterTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("DeleteFdbClusterTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Fdb.Nodes))
	for i, node := range r.Cfg.Services.Fdb.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(rmContainerStep) },
		},
	})
}
