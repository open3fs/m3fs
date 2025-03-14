package monitor

import (
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/task"
)

// CreateMonitorTask is a task for creating a 3fs monitor.
type CreateMonitorTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *CreateMonitorTask) Init(r *task.Runtime) {
	t.BaseTask.Init(r)
	t.BaseTask.SetName("CreateMonitorTask")
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
	})
}

// DeleteMonitorTask is a task for deleting a 3fs monitor.
type DeleteMonitorTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *DeleteMonitorTask) Init(r *task.Runtime) {
	t.BaseTask.Init(r)
	t.BaseTask.SetName("DeleteMonitorTask")
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
