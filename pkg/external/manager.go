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

package external

import (
	"context"
	"sync"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/log"
)

type externalInterface interface {
	init(em *Manager, logger log.Interface)
}

type externalBase struct {
	em     *Manager
	logger log.Interface
}

func (eb *externalBase) init(em *Manager, logger log.Interface) {
	eb.em = em
	eb.logger = logger
}

func (eb *externalBase) run(ctx context.Context, cmdName string, args ...string) (string, error) {
	anyArgs := []any{}
	for _, arg := range args {
		anyArgs = append(anyArgs, arg)
	}

	cmd := NewCommand(cmdName, anyArgs...)
	cmd.runner = eb.em.Runner
	out, err := cmd.SudoExec(ctx)
	if err != nil {
		return out, errors.Annotatef(err, "sudo run cmd [%s]", cmd.String())
	}
	return out, nil

}

// create a new external
type newExternalFunc func() externalInterface

var (
	newExternals []newExternalFunc
	lock         sync.Mutex
)

func registerNewExternalFunc(f newExternalFunc) {
	lock.Lock()
	defer lock.Unlock()
	newExternals = append(newExternals, f)
}

// Manager provides a way to use all external interfaces
type Manager struct {
	Runner RunnerInterface

	Os     OsInterface
	Net    NetInterface
	Docker DockerInterface
	Disk   DiskInterface
	FS     FSInterface
}

// NewManagerFunc type of new manager func.
type NewManagerFunc func() *Manager

// NewManager create a new external manager
func NewManager(runner RunnerInterface, logger log.Interface) (em *Manager) {
	em = &Manager{
		Runner: runner,
	}
	for _, newExternal := range newExternals {
		newExternal().init(em, logger)
	}
	return em
}

var remoteManagerCache sync.Map

// NewRemoteRunnerManager create a new remote runner manager
func NewRemoteRunnerManager(node *config.Node, logger log.Interface) (*Manager, error) {
	mgr, ok := remoteManagerCache.Load(node)
	if ok {
		return mgr.(*Manager), nil
	}
	runner, err := NewRemoteRunner(&RemoteRunnerCfg{
		Username:   node.Username,
		Password:   node.Password,
		TargetHost: node.Host,
		TargetPort: node.Port,
		Logger:     logger,
		// TODO: add timeout config
	})
	if err != nil {
		return nil, errors.Annotatef(err, "create remote runner for node [%s]", node.Name)
	}

	return NewManager(runner, logger), nil
}
