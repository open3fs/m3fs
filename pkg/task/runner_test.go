package task

import (
	"testing"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/stretchr/testify/mock"
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
	s.runner = NewRunner(new(config.Config), s.mockTask)
}

func (s *runnerSuite) TestInit() {
	s.mockTask.On("Init", mock.AnythingOfType("*task.Runtime"))

	s.runner.Init()

	s.mockTask.AssertExpectations(s.T())
}

func (s *runnerSuite) TestRegiserAfterInit() {
	s.TestInit()

	s.Error(s.runner.Register(s.mockTask), "runner has been initialized")
}

func (s *runnerSuite) TestRegiser() {
	task2 := new(mockTask)
	s.NoError(s.runner.Register(s.mockTask))

	s.Equal(s.runner.tasks, []Interface{s.mockTask, task2})
}

func (s *runnerSuite) TestRun() {
	s.mockTask.On("Name").Return("mockTask")
	s.mockTask.On("Run").Return(nil)

	s.runner.Run(s.Ctx())

	s.mockTask.AssertExpectations(s.T())
}
