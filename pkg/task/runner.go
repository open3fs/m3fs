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
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/utils"
)

// defines keys of runtime cache.
const (
	RuntimeArtifactTmpDirKey    = "artifact/tmp_dir"
	RuntimeArtifactPathKey      = "artifact/path"
	RuntimeArtifactGzipKey      = "artifact/gzip"
	RuntimeArtifactSha256sumKey = "artifact/sha256sum"
	RuntimeArtifactFilePathsKey = "artifact/file_paths"

	RuntimeFdbClusterFileContentKey = "fdb_cluster_file_content"
	RuntimeMgmtdServerAddressesKey  = "mgmtd_server_addresses"
	RuntimeUserTokenKey             = "user_token"
	RuntimeAdminCliTomlKey          = "admin_cli_toml"
)

// Runtime contains task run info
type Runtime struct {
	sync.Map
	Cfg       *config.Config
	Nodes     map[string]config.Node
	Services  *config.Services
	WorkDir   string
	LocalEm   *external.Manager
	LocalNode *config.Node
}

// LoadString load string value form sync map
func (r *Runtime) LoadString(key any) (string, bool) {
	valI, ok := r.Load(key)
	if !ok {
		return "", false
	}

	return valI.(string), true
}

// LoadBool load bool value form sync map
func (r *Runtime) LoadBool(key any) (bool, bool) {
	valI, ok := r.Load(key)
	if !ok {
		return false, false
	}

	return valI.(bool), true
}

// LoadInt load int value form sync map
func (r *Runtime) LoadInt(key any) (int, bool) {
	valI, ok := r.Load(key)
	if !ok {
		return 0, false
	}

	return valI.(int), true
}

// Runner is a task runner.
type Runner struct {
	Runtime   *Runtime
	tasks     []Interface
	cfg       *config.Config
	localNode *config.Node
	init      bool
}

// Init initializes all tasks.
func (r *Runner) Init() {
	r.Runtime = &Runtime{Cfg: r.cfg, WorkDir: r.cfg.WorkDir, LocalNode: r.localNode}
	r.Runtime.Nodes = make(map[string]config.Node, len(r.cfg.Nodes))
	for _, node := range r.cfg.Nodes {
		r.Runtime.Nodes[node.Name] = node
	}
	r.Runtime.Services = &r.cfg.Services
	logger := log.Logger.Subscribe(log.FieldKeyNode, "<LOCAL>")
	runnerCfg := &external.LocalRunnerCfg{
		Logger:         logger,
		MaxExitTimeout: r.cfg.CmdMaxExitTimeout,
	}
	if r.localNode != nil {
		runnerCfg.User = r.localNode.Username
		if r.localNode.Password != nil {
			runnerCfg.Password = *r.localNode.Password
		}
	}
	em := external.NewManager(external.NewLocalRunner(runnerCfg), logger)
	r.Runtime.LocalEm = em

	for _, task := range r.tasks {
		task.Init(r.Runtime, log.Logger.Subscribe(log.FieldKeyTask, task.Name()))
	}
	r.init = true
}

// Store sets the value for a key.
func (r *Runner) Store(key, value any) error {
	if r.Runtime == nil {
		return errors.Errorf("Runtime hasn't been initialized")
	}
	r.Runtime.Store(key, value)
	return nil
}

// Register registers tasks.
func (r *Runner) Register(task ...Interface) error {
	if r.init {
		return errors.New("runner has been initialized")
	}
	r.tasks = append(r.tasks, task...)
	return nil
}

// Run runs all tasks.
func (r *Runner) Run(ctx context.Context) error {
	for _, task := range r.tasks {
		logrus.Infof("Running task %s", task.Name())
		if err := task.Run(ctx); err != nil {
			return errors.Annotatef(err, "run task %s", task.Name())
		}
	}

	return nil
}

// NewRunner creates a new task runner.
func NewRunner(cfg *config.Config, tasks ...Interface) (*Runner, error) {
	localIPs, err := utils.GetLocalIPs()
	if err != nil {
		return nil, errors.Trace(err)
	}
	var localNode *config.Node
	for i, node := range cfg.Nodes {
		if isLocal, err := utils.IsLocalHost(node.Host, localIPs); err != nil {
			return nil, errors.Trace(err)
		} else if isLocal {
			localNode = &cfg.Nodes[i]
			break
		}
	}
	return &Runner{
		tasks:     tasks,
		localNode: localNode,
		cfg:       cfg,
	}, nil
}
