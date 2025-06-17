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
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
	texternal "github.com/open3fs/m3fs/tests/external"
	tmodel "github.com/open3fs/m3fs/tests/model"
)

// StepSuite is the base Suite for all step suites.
type StepSuite struct {
	tmodel.Suite

	Cfg        *config.Config
	Runtime    *task.Runtime
	MockEm     *external.Manager
	MockRunner *texternal.MockRunner
	MockDocker *texternal.MockDocker
	MockDisk   *texternal.MockDisk
	MockFS     *texternal.MockFS
	// NOTE: external.FSInterface is not implemented for remote runner.
	// MockFS          *texternal.MockFS
	MockLocalEm     *external.Manager
	MockLocalRunner *texternal.MockRunner
	MockLocalFS     *texternal.MockFS
	MockLocalDocker *texternal.MockDocker
}

// SetupTest runs before each test in the step suite.
func (s *StepSuite) SetupTest() {
	s.Suite.SetupTest()

	s.Cfg = config.NewConfigWithDefaults()
	s.Cfg.Name = "test-cluster"
	s.Cfg.WorkDir = "/root/3fs"

	s.MockRunner = new(texternal.MockRunner)
	s.MockDocker = new(texternal.MockDocker)
	s.MockDisk = new(texternal.MockDisk)
	s.MockFS = new(texternal.MockFS)
	s.MockEm = &external.Manager{
		Runner: s.MockRunner,
		Docker: s.MockDocker,
		FS:     s.MockFS,
		Disk:   s.MockDisk,
	}

	s.MockLocalDocker = new(texternal.MockDocker)
	s.MockLocalRunner = new(texternal.MockRunner)
	s.MockLocalFS = new(texternal.MockFS)
	s.MockLocalEm = &external.Manager{
		Runner: s.MockLocalRunner,
		FS:     s.MockLocalFS,
		Docker: s.MockLocalDocker,
	}

	s.SetupRuntime()
}

// SetupRuntime setup runtime with the test config.
func (s *StepSuite) SetupRuntime() {
	s.Runtime = &task.Runtime{
		Cfg:      s.Cfg,
		WorkDir:  s.Cfg.WorkDir,
		Services: &s.Cfg.Services,
		LocalEm:  s.MockLocalEm,
	}
	s.Runtime.Nodes = make(map[string]config.Node, len(s.Cfg.Nodes))
	nodesMap := make(map[string]*model.Node, len(s.Cfg.Nodes))
	db := s.NewDB()
	for _, node := range s.Cfg.Nodes {
		s.Runtime.Nodes[node.Name] = node
		nodeModel := &model.Node{
			Name: node.Name,
			Host: node.Host,
		}
		if s.CreateTables {
			s.NoError(db.Create(nodeModel).Error)
			nodesMap[node.Name] = nodeModel
		}
	}
	s.Runtime.Store(task.RuntimeNodesMapKey, nodesMap)
	s.Runtime.Store(task.RuntimeDbKey, db)
	s.Runtime.Services = &s.Cfg.Services
}
