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

package pg

import (
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/task"
	"github.com/open3fs/m3fs/pkg/utils"
)

// CreatePgClusterTask is a task for creating a new pg cluster.
type CreatePgClusterTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *CreatePgClusterTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("CreatePgClusterTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Pg.Nodes))
	for i, node := range r.Cfg.Services.Pg.Nodes {
		nodes[i] = r.Nodes[node]
	}
	replicaName := "open3fs-replica-" + utils.RandomString(4)
	replicaPassword := utils.RandomString(16)
	t.SetSteps([]task.StepConfig{
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: func() task.Step {
				return &runContainerStep{
					masterNodeName:  nodes[0].Name,
					replicaUser:     replicaName,
					replicaPassword: replicaPassword,
				}
			},
		},
	})
}

// DeletePgClusterTask is a task for deleting a pg cluster.
type DeletePgClusterTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *DeletePgClusterTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("DeletePgClusterTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Pg.Nodes))
	for i, node := range r.Cfg.Services.Pg.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(rmContainerStep) },
		},
	})
}
