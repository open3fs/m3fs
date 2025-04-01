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

	"github.com/fatih/color"
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

func (s *runnerSuite) TestInitWithIB() {
	s.runner.cfg.NetworkType = config.NetworkTypeIB
	s.TestInit()

	s.Equal(s.runner.Runtime.MgmtdProtocol, "IPoIB")
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

func (s *runnerSuite) TestTaskInfoHighlighting() {
	cfg := &config.Config{
		UI: config.UIConfig{
			TaskInfoColor: "green",
		},
	}

	s.runner.cfg = cfg
	s.mockTask.On("Name").Return("mockTask")
	s.mockTask.On("Run").Return(nil)

	s.NoError(s.runner.Run(s.Ctx()))

	s.mockTask.AssertExpectations(s.T())

	noColorCfg := &config.Config{
		UI: config.UIConfig{
			TaskInfoColor: "none",
		},
	}

	s.runner.cfg = noColorCfg
	s.mockTask.On("Name").Return("mockTask")
	s.mockTask.On("Run").Return(nil)

	s.NoError(s.runner.Run(s.Ctx()))

	s.mockTask.AssertExpectations(s.T())

	invalidColorCfg := &config.Config{
		UI: config.UIConfig{
			TaskInfoColor: "invalid-color",
		},
	}

	s.runner.cfg = invalidColorCfg
	s.mockTask.On("Name").Return("mockTask")
	s.mockTask.On("Run").Return(nil)

	s.NoError(s.runner.Run(s.Ctx()))

	s.mockTask.AssertExpectations(s.T())

	emptyColorCfg := &config.Config{
		UI: config.UIConfig{
			TaskInfoColor: "",
		},
	}

	s.runner.cfg = emptyColorCfg
	s.mockTask.On("Name").Return("mockTask")
	s.mockTask.On("Run").Return(nil)

	s.NoError(s.runner.Run(s.Ctx()))

	s.mockTask.AssertExpectations(s.T())

	noUICfg := &config.Config{}

	s.runner.cfg = noUICfg
	s.mockTask.On("Name").Return("mockTask")
	s.mockTask.On("Run").Return(nil)

	s.NoError(s.runner.Run(s.Ctx()))

	s.mockTask.AssertExpectations(s.T())
}

func (s *runnerSuite) TestGetColorAttribute() {
	s.Equal(color.FgHiGreen, getColorAttribute("green"))
	s.Equal(color.FgHiCyan, getColorAttribute("cyan"))
	s.Equal(color.FgHiYellow, getColorAttribute("yellow"))
	s.Equal(color.FgHiBlue, getColorAttribute("blue"))
	s.Equal(color.FgHiMagenta, getColorAttribute("magenta"))
	s.Equal(color.FgHiRed, getColorAttribute("red"))
	s.Equal(color.FgHiWhite, getColorAttribute("white"))

	s.Equal(color.FgHiGreen, getColorAttribute("GREEN"))
	s.Equal(color.FgHiCyan, getColorAttribute("Cyan"))

	s.Equal(color.Attribute(-1), getColorAttribute("none"))
	s.Equal(color.Attribute(-1), getColorAttribute("NONE"))

	s.Equal(color.Attribute(-1), getColorAttribute("invalid-color"))
	s.Equal(color.Attribute(-1), getColorAttribute(""))
}
