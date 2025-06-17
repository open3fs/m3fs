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

package task

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/log"
)

// Interface defines the interface that all tasks must implement.
type Interface interface {
	Init(*Runtime, log.Interface)
	Name() string
	Run(context.Context) error
	SetSteps([]StepConfig)
}

// BaseTask is a base struct that all tasks should embed.
type BaseTask struct {
	name    string
	Runtime *Runtime
	steps   []StepConfig
	Logger  log.Interface
}

// Init initializes the task with the external manager and the configuration.
func (t *BaseTask) Init(r *Runtime, parentLogger log.Interface) {
	t.Runtime = r
	t.Logger = parentLogger.Subscribe(log.FieldKeyTask, t.Name())
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

func (t *BaseTask) newStepExecuter(newStepFunc func() Step, retryTime int) func(context.Context, config.Node) error {
	return func(ctx context.Context, node config.Node) error {
		step := newStepFunc()
		logger := t.Logger.Subscribe(log.FieldKeyNode, node.Name)

		var em *external.Manager
		var err error
		if t.Runtime.LocalNode != nil && node.Name == t.Runtime.LocalNode.Name {
			em = t.Runtime.LocalEm
		} else {
			em, err = external.NewRemoteRunnerManager(&node, logger)
			if err != nil {
				return errors.Trace(err)
			}
		}
		step.Init(t.Runtime, em, node, logger)
		for i := 0; i <= retryTime; i++ {
			err = step.Execute(ctx)
			if err != nil && i != retryTime {
				logger.Warnf("Step failed, retrying: %v", err)
				time.Sleep(time.Second)
				continue
			}
			break
		}
		return errors.Trace(err)
	}
}

// ExecuteSteps executes all the steps of the task.
func (t *BaseTask) ExecuteSteps(ctx context.Context) error {
	for _, stepCfg := range t.steps {
		executor := t.newStepExecuter(stepCfg.NewStep, stepCfg.RetryTime)
		if stepCfg.Parallel && len(stepCfg.Nodes) > 1 {
			workerPool := common.NewWorkerPool(executor, len(stepCfg.Nodes))
			workerPool.Start(ctx)
			for _, node := range stepCfg.Nodes {
				workerPool.Add(node)
			}
			workerPool.Join()
			errs := workerPool.Errors()
			if len(errs) > 0 {
				if logrus.StandardLogger().Level == logrus.DebugLevel {
					errorsTrace := make([]string, len(errs))
					for _, err := range errs {
						errorsTrace = append(errorsTrace, errors.StackTrace(err))
					}
					logrus.Debugf("Run step failed, output: %s", strings.Join(errorsTrace, "\n"))
				}
				return errors.Trace(errs[0])
			}
		} else {
			for _, node := range stepCfg.Nodes {
				var err error
				for i := 0; i <= stepCfg.RetryTime; i++ {
					if err = executor(ctx, node); err != nil && i != stepCfg.RetryTime {
						t.Logger.Warnf("Step failed, retrying: %v", err)
						time.Sleep(time.Second)
						continue
					}
					break
				}
				if err != nil {
					return errors.Trace(err)
				}
			}
		}
	}
	return nil
}

// Step is an interface that defines the methods that all steps must implement,
// in order to be executed by the task.
type Step interface {
	Init(r *Runtime, em *external.Manager, node config.Node, logger log.Interface)
	Execute(context.Context) error
}

// StepConfig is a struct that holds the configuration of a step.
type StepConfig struct {
	Nodes     []config.Node
	Parallel  bool
	RetryTime int
	NewStep   func() Step
}

// BaseStep is a base struct that all steps should embed.
type BaseStep struct {
	Em      *external.Manager
	Runtime *Runtime
	Node    config.Node
	Logger  log.Interface
}

// GetNodeKey returns key for the node.
func (s *BaseStep) GetNodeKey(key string) string {
	return fmt.Sprintf("%s/%s", key, s.Node.Name)
}

// GetErdmaSoPathKey returns the key for the rdma so file for the node.
func (s *BaseStep) GetErdmaSoPathKey() string {
	return fmt.Sprintf("%s-erdma-so", s.Node.Host)
}

// GetNodeModelID returns the model ID of the node.
func (s *BaseStep) GetNodeModelID() uint {
	return s.Runtime.LoadNodesMap()[s.Node.Name].ID
}

// Init initializes the step with the external manager and the configuration.
func (s *BaseStep) Init(r *Runtime, em *external.Manager, node config.Node, logger log.Interface) {
	s.Em = em
	s.Runtime = r
	s.Logger = logger
	s.Node = node
}

// Execute is a no-op implementation of the Execute method, which should be
// overridden by the steps that embed the BaseStep struct.
func (s *BaseStep) Execute(context.Context) error {
	return nil
}

// GetErdmaSoPath returns the path of the erdma so file.
func (s *BaseStep) GetErdmaSoPath(ctx context.Context) error {
	if s.Runtime.Cfg.NetworkType != config.NetworkTypeERDMA {
		return nil
	}
	erdmaSoKey := s.GetErdmaSoPathKey()
	if _, ok := s.Runtime.Load(erdmaSoKey); ok {
		return nil
	}
	output, err := s.Em.Runner.Exec(ctx,
		"readlink", "-f", "/usr/lib/x86_64-linux-gnu/libibverbs/liberdma-rdmav34.so")
	if err != nil {
		return errors.Annotatef(err, "readlink -f /usr/lib/x86_64-linux-gnu/libibverbs/liberdma-rdmav34.so")
	}
	s.Runtime.Store(erdmaSoKey, strings.TrimSpace(output))
	return nil
}

// GetRdmaVolumes returns the volumes need mapped to container for the rdma network.
func (s *BaseStep) GetRdmaVolumes() []*external.VolumeArgs {
	volumes := []*external.VolumeArgs{}
	if s.Runtime.Cfg.NetworkType == config.NetworkTypeIB {
		return volumes
	}

	if s.Runtime.Cfg.NetworkType != config.NetworkTypeRDMA {
		ibdev2netdevScriptPath := path.Join(s.Runtime.Cfg.WorkDir, "bin", "ibdev2netdev")
		volumes = append(volumes, &external.VolumeArgs{
			Source: ibdev2netdevScriptPath,
			Target: "/usr/sbin/ibdev2netdev",
		})
	}
	if s.Runtime.Cfg.NetworkType == config.NetworkTypeERDMA {
		erdmaSoPath, ok := s.Runtime.Load(s.GetErdmaSoPathKey())
		if ok {
			volumes = append(volumes, []*external.VolumeArgs{
				{
					Source: "/etc/libibverbs.d/erdma.driver",
					Target: "/etc/libibverbs.d/erdma.driver",
				},
				{
					Source: erdmaSoPath.(string),
					Target: "/usr/lib/x86_64-linux-gnu/libibverbs/liberdma-rdmav34.so",
				},
			}...)
		}
	}
	return volumes
}

// LocalStep is an interface that defines the methods that all local steps must implement,
type LocalStep interface {
	Init(*Runtime, log.Interface)
	Execute(context.Context) error
}

// BaseLocalStep is a base local step.
type BaseLocalStep struct {
	Runtime *Runtime
	Logger  log.Interface
}

// Init initializes a base local step.
func (s *BaseLocalStep) Init(r *Runtime, logger log.Interface) {
	s.Runtime = r
	// TODO: add step name
	s.Logger = logger
}
