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

package monitor

import (
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
	ttask "github.com/open3fs/m3fs/tests/task"
)

var suiteRun = suite.Run

func TestGenMonitorConfigStep(t *testing.T) {
	suiteRun(t, &genMonitorConfigStepSuite{})
}

type genMonitorConfigStepSuite struct {
	ttask.StepSuite

	step *genMonitorConfigStep
}

func (s *genMonitorConfigStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &genMonitorConfigStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *genMonitorConfigStepSuite) Test() {
	s.MockLocalFS.On("MkdirTemp", os.TempDir(), "3fs-monitor").Return("/tmp/3fs-monitor.xxx", nil)
	s.MockLocalFS.On("WriteFile", "/tmp/3fs-monitor.xxx/monitor_collector_main.toml",
		mock.AnythingOfType("[]uint8"), os.FileMode(0644)).Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))

	tmpDirValue, ok := s.Runtime.Load(task.RuntimeMonitorTmpDirKey)
	s.True(ok)
	tmpDir := tmpDirValue.(string)
	s.Equal("/tmp/3fs-monitor.xxx", tmpDir)
}

func TestRunContainerStep(t *testing.T) {
	suiteRun(t, &runContainerStepSuite{})
}

type runContainerStepSuite struct {
	ttask.StepSuite

	step *runContainerStep
}

func (s *runContainerStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &runContainerStep{}
	s.Cfg.Nodes = append(s.Cfg.Nodes, config.Node{
		Name: "test-node",
		Host: "1.1.1.1",
	})
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
	s.Runtime.Store(task.RuntimeMonitorTmpDirKey, "/tmp/3f-monitor.xxx")
}

func (s *runContainerStepSuite) Test() {
	etcDir := "/root/3fs/monitor/etc"
	logDir := "/root/3fs/monitor/log"
	s.MockFS.On("MkdirAll", etcDir).Return(nil)
	s.MockFS.On("MkdirAll", logDir).Return(nil)
	s.MockRunner.On("Scp", "/tmp/3f-monitor.xxx/monitor_collector_main.toml",
		"/root/3fs/monitor/etc/monitor_collector_main.toml").Return(nil)
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageName3FS)
	s.NoError(err)
	args := &external.RunArgs{
		Image:         img,
		Name:          common.Pointer("3fs-monitor"),
		HostNetwork:   true,
		RestartPolicy: external.ContainerRestartPolicyUnlessStepped,
		Privileged:    common.Pointer(true),
		Detach:        common.Pointer(true),
		Volumes: []*external.VolumeArgs{
			{
				Source: "/dev",
				Target: "/dev",
			},
			{
				Source: etcDir,
				Target: "/opt/3fs/etc",
			},
			{
				Source: logDir,
				Target: "/var/log/3fs",
			},
		},
		Command: []string{
			"/opt/3fs/bin/monitor_collector_main",
			"--cfg",
			"/opt/3fs/etc/monitor_collector_main.toml",
		},
	}
	s.Runtime.Store(s.step.GetErdmaSoPathKey(),
		"/usr/lib/x86_64-linux-gnu/libibverbs/liberdma-rdmav34.so")
	args.Volumes = append(args.Volumes, s.step.GetRdmaVolumes()...)
	s.MockDocker.On("Run", args).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	var monService model.MonService
	s.NoError(s.NewDB().Model(new(model.MonService)).First(&monService).Error)
	monServiceExp := model.MonService{
		Model:  monService.Model,
		Name:   s.Runtime.Services.Monitor.ContainerName,
		NodeID: s.Runtime.LoadNodesMap()[s.step.Node.Name].ID,
	}
	s.Equal(monServiceExp, monService)

	s.MockRunner.AssertExpectations(s.T())
	s.MockFS.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func TestRmContainerStep(t *testing.T) {
	suiteRun(t, &rmContainerStepSuite{})
}

type rmContainerStepSuite struct {
	ttask.StepSuite

	step   *rmContainerStep
	etcDir string
	logDir string
}

func (s *rmContainerStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &rmContainerStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
	s.etcDir = "/root/3fs/monitor/etc"
	s.logDir = "/root/3fs/monitor/log"
}

func (s *rmContainerStepSuite) TestRmContainerStep() {
	s.MockDocker.On("Rm", s.Cfg.Services.Monitor.ContainerName, true).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.etcDir}).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.logDir}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}
