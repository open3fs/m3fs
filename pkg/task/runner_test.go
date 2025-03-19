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
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/open3fs/m3fs/pkg/config"
)

func TestRunnerSuite(t *testing.T) {
	suiteRun(t, new(runnerSuite))
}

type runnerSuite struct {
	baseSuite
	runner   *Runner
	mockTask *mockTask
}

func (s *runnerSuite) SetupTest() {
	s.baseSuite.SetupTest()
	s.mockTask = new(mockTask)
	s.runner = &Runner{
		tasks: []Interface{s.mockTask},
		cfg:   new(config.Config),
	}
}

func (s *runnerSuite) TestInit() {
	s.mockTask.On("Init", mock.AnythingOfType("*task.Runtime"))
	s.mockTask.On("Name").Return("mockTask")

	s.runner.Init()

	s.mockTask.AssertExpectations(s.T())
}

func (s *runnerSuite) TestRegisterAfterInit() {
	s.TestInit()
	s.mockTask.On("Name").Return("mockTask")

	s.Error(s.runner.Register(s.mockTask), "runner has been initialized")
}

func (s *runnerSuite) TestRegister() {
	task2 := new(mockTask)
	s.NoError(s.runner.Register(s.mockTask))

	s.Equal(s.runner.tasks, []Interface{s.mockTask, task2})
}

func (s *runnerSuite) TestRun() {
	s.mockTask.On("Name").Return("mockTask")
	s.mockTask.On("Run").Return(nil)

	s.NoError(s.runner.Run(s.Ctx()))

	s.mockTask.AssertExpectations(s.T())
}
