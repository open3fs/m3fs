package task

import (
	"context"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/sirupsen/logrus"
)

// Interface defines the interface that all tasks must implement.
type Interface interface {
	Init(*Runtime)
	Name() string
	Run(context.Context) error
	SetSteps([]StepConfig)
}

// BaseTask is a base struct that all tasks should embed.
type BaseTask struct {
	name    string
	Runtime *Runtime
	steps   []StepConfig
}

// Init initializes the task with the external manager and the configuration.
func (t *BaseTask) Init(r *Runtime) {
	t.Runtime = r
}

// Run runs task steps
func (t *BaseTask) Run(ctx context.Context) error {
	return t.ExecuteSteps(ctx)
}

// SetSteps sets the steps of the task.
func (t *BaseTask) SetSteps(steps []StepConfig) {
	t.steps = steps
}

// SetName sets the name of the task.
func (t *BaseTask) SetName(name string) {
	t.name = name
}

// Name returns the name of the task.
func (t *BaseTask) Name() string {
	return t.name
}

// ExecuteSteps executes all the steps of the task.
func (t *BaseTask) ExecuteSteps(ctx context.Context) error {
	// TODO: support parallel steps
	for _, stepCfg := range t.steps {
		for _, node := range stepCfg.Nodes {
			step := stepCfg.NewStep()
			em, err := external.NewRemoteRunnerManager(&node)
			if err != nil {
				return errors.Trace(err)
			}
			step.Init(t.Runtime, em)
			if err := step.Execute(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

// Step is an interface that defines the methods that all steps must implement,
// in order to be executed by the task.
type Step interface {
	Init(r *Runtime, em *external.Manager)
	Execute(context.Context) error
}

// StepConfig is a struct that holds the configuration of a step.
type StepConfig struct {
	Nodes    []config.Node
	Parallel bool
	NewStep  func() Step
}

// BaseStep is a base struct that all steps should embed.
type BaseStep struct {
	Em      *external.Manager
	Runtime *Runtime
	Logger  *logrus.Logger
}

// Init initializes the step with the external manager and the configuration.
func (s *BaseStep) Init(r *Runtime, em *external.Manager) {
	s.Em = em
	s.Runtime = r
	s.Logger = logrus.StandardLogger()
}

// Execute is a no-op implementation of the Execute method, which should be
// overridden by the steps that embed the BaseStep struct.
func (s *BaseStep) Execute(context.Context) error {
	return nil
}
