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
	"os"
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

// RunAdminCli run admin cli in container
func (s *BaseStep) RunAdminCli(ctx context.Context, container, cmd string) (string, error) {
	out, err := s.Em.Docker.Exec(ctx, container,
		"/opt/3fs/bin/admin_cli", "-cfg", "/opt/3fs/etc/admin_cli.toml", fmt.Sprintf("%q", cmd))
	if err != nil {
		return "", errors.Trace(err)
	}

	return out, nil
}

// WriteRemoteFile write file to remote path
func (s *BaseStep) WriteRemoteFile(ctx context.Context, remotePath string, data []byte) error {

	localTmpFile, err := s.Runtime.LocalEm.FS.MkTempFile(ctx, os.TempDir())
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err = s.Runtime.LocalEm.FS.RemoveAll(ctx, localTmpFile); err != nil {
			s.Logger.Warnf("Failed to delete tmp file %s:%s", localTmpFile, err)
		}
	}()

	baseDir := path.Dir(remotePath)
	if err := s.Em.FS.MkdirAll(ctx, baseDir); err != nil {
		return errors.Trace(err)
	}

	err = s.Runtime.LocalEm.FS.WriteFile(localTmpFile, data, 0644)
	if err != nil {
		return errors.Trace(err)
	}

	if err = s.Em.Runner.Scp(ctx, localTmpFile, remotePath); err != nil {
		return errors.Trace(err)
	}

	return nil
}

// CreateScriptAndService create service and its script
func (s *BaseStep) CreateScriptAndService(
	ctx context.Context, scriptName, serviceName string, scriptBytes, serviceBytes []byte) error {

	s.Logger.Infof("Creating %s script and %s service...", scriptName, serviceName)
	scriptPath := path.Join(s.Runtime.WorkDir, "bin", scriptName)
	err := s.WriteRemoteFile(ctx, scriptPath, scriptBytes)
	if err != nil {
		return errors.Annotatef(err, "write remote file %s", scriptPath)
	}
	_, err = s.Em.Runner.Exec(ctx, "chmod", "+x", scriptPath)
	if err != nil {
		return errors.Trace(err)
	}

	servicePath := path.Join(s.Runtime.Cfg.ServiceBasePath, serviceName)
	if err = s.WriteRemoteFile(ctx, servicePath, serviceBytes); err != nil {
		return errors.Annotatef(err, "write remote file %s", servicePath)
	}

	if _, err = s.Em.Runner.Exec(ctx, "systemctl", "enable", serviceName); err != nil {
		return errors.Annotatef(err, "enable %s", serviceName)
	}

	if _, err = s.Em.Runner.Exec(ctx, "systemctl", "daemon-reload"); err != nil {
		return errors.Annotate(err, "daemon reload")
	}

	return nil
}

// DeleteService delete system service
func (s *BaseStep) DeleteService(ctx context.Context, serviceName string) error {
	servicePath := path.Join(s.Runtime.Cfg.ServiceBasePath, serviceName)
	notExists, err := s.Em.FS.IsNotExist(servicePath)
	if err != nil {
		return errors.Trace(err)
	}
	if notExists {
		return nil
	}

	if _, err := s.Em.Runner.Exec(ctx, "systemctl", "disable", serviceName); err != nil {
		return errors.Annotatef(err, "disable %s", serviceName)
	}

	if err := s.Em.FS.RemoveAll(ctx, servicePath); err != nil {
		return errors.Trace(err)
	}

	if _, err := s.Em.Runner.Exec(ctx, "systemctl", "daemon-reload"); err != nil {
		return errors.Annotate(err, "daemon reload")
	}

	return nil
}

// GetOsName returns the distribution name of the os.
func (s *BaseStep) GetOsName(ctx context.Context) (string, error) {
	if osName, ok := s.Runtime.Load(RuntimeOsNameKey); ok {
		return osName.(string), nil
	}
	output, err := s.Em.Runner.Exec(ctx, "bash", "-c", `'. /etc/os-release && echo $NAME'`)
	if err != nil {
		return "", errors.Annotatef(err, `bash -c '. /etc/os-release && echo $NAME'`)
	}
	osName := strings.TrimSpace(output)
	s.Runtime.Store(RuntimeOsNameKey, osName)
	return osName, nil
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
