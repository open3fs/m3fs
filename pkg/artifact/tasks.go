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

package artifact

import (
	"context"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/task"
)

// ExportArtifactTask is a task for exporting a 3fs artifact.
type ExportArtifactTask struct {
	task.BaseTask

	localSteps []task.LocalStep
}

// Init initializes the task.
func (t *ExportArtifactTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("ExportArtifactTask")
	t.BaseTask.Init(r, logger)
	t.localSteps = []task.LocalStep{
		new(prepareTmpDirStep),
		new(downloadImagesStep),
		new(tarFilesStep),
	}
}

// Run runs task steps
func (t *ExportArtifactTask) Run(ctx context.Context) error {
	for _, step := range t.localSteps {
		step.Init(t.Runtime, log.Logger.Subscribe(log.FieldKeyNode, "<LOCAL>"))
		if err := step.Execute(ctx); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// ImportArtifactTask is a task for importing the 3fs artifact.
type ImportArtifactTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *ImportArtifactTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("ImportArtifactTask")
	t.BaseTask.Init(r, logger)
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   []config.Node{r.Cfg.Nodes[0]},
			NewStep: func() task.Step { return new(sha256sumArtifactStep) },
		},
		{
			Nodes:    r.Cfg.Nodes,
			Parallel: true,
			NewStep:  func() task.Step { return new(distributeArtifactStep) },
		},
		{
			Nodes:    r.Cfg.Nodes,
			Parallel: true,
			NewStep:  func() task.Step { return new(importArtifactStep) },
		},
		{
			Nodes:    r.Cfg.Nodes,
			Parallel: true,
			NewStep:  func() task.Step { return new(removeArtifactStep) },
		},
	})
}
