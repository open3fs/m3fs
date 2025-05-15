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

package grafana

import (
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/task"
)

// CreateGrafanaServiceTask is a task for creating a new grafana service.
type CreateGrafanaServiceTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *CreateGrafanaServiceTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("CreateGrafanaServiceTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Grafana.Nodes))
	for i, node := range r.Cfg.Services.Grafana.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(genGrafanaYamlStep) },
		},
		{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(startContainerStep) },
		},
	})
}

// DeleteGrafanaServiceTask is a task for deleting a grafana service.
type DeleteGrafanaServiceTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *DeleteGrafanaServiceTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("DeleteGrafanaServiceTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Grafana.Nodes))
	for i, node := range r.Cfg.Services.Grafana.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(rmContainerStep) },
		},
	})
}
