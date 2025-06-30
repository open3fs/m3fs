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

package fdb

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
	ttask "github.com/open3fs/m3fs/tests/task"
)

var suiteRun = suite.Run

func TestGenClusterFileContentStep(t *testing.T) {
	suiteRun(t, &genClusterFileContentStepSuite{})
}

type genClusterFileContentStepSuite struct {
	ttask.StepSuite

	step *genClusterFileContentStep
}

func (s *genClusterFileContentStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &genClusterFileContentStep{}
	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
		{
			Name: "node2",
			Host: "1.1.1.2",
		},
	}
	s.Cfg.Services.Fdb.Nodes = []string{"node1", "node2"}
	s.Cfg.Services.Fdb.Port = 4500
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
}

func (s *genClusterFileContentStepSuite) TestGenClusterFileContentStep() {
	s.NoError(s.step.Execute(s.Ctx()))

	contentI, ok := s.Runtime.Load(task.RuntimeFdbClusterFileContentKey)
	s.True(ok)
	s.True(strings.Contains(contentI.(string), "@1.1.1.1:4500,1.1.1.2:4500"))
}

func TestRunContainerStep(t *testing.T) {
	suiteRun(t, &runContainerStepSuite{})
}

type runContainerStepSuite struct {
	ttask.StepSuite

	step    *runContainerStep
	dataDir string
	logDir  string
}

func (s *runContainerStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &runContainerStep{}
	s.dataDir = "/root/3fs/fdb/data"
	s.logDir = "/root/3fs/fdb/logs"
	s.Cfg.Nodes = append(s.Cfg.Nodes, config.Node{
		Name: "test-node",
		Host: "1.1.1.1",
	})
	s.Cfg.Services.Fdb.Nodes = []string{"test-node"}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
	s.Runtime.Store(task.RuntimeFdbClusterFileContentKey, "xxxx")
}

func (s *runContainerStepSuite) TestRunContainerStep() {
	s.MockFS.On("MkdirAll", s.dataDir).Return(nil)
	s.MockFS.On("MkdirAll", s.logDir).Return(nil)
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageNameFdb)
	s.NoError(err)
	s.MockDocker.On("Run", &external.RunArgs{
		Image:         img,
		RestartPolicy: external.ContainerRestartPolicyUnlessStopped,
		Name:          &s.Cfg.Services.Fdb.ContainerName,
		HostNetwork:   true,
		Detach:        common.Pointer(true),
		Envs: map[string]string{
			"FDB_CLUSTER_FILE_CONTENTS": "xxxx",
		},
		Volumes: []*external.VolumeArgs{
			{
				Source: s.dataDir,
				Target: "/var/fdb/data",
			},
			{
				Source: s.logDir,
				Target: "/var/fdb/logs",
			},
		},
	}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	var fdbClusterDB model.FdbCluster
	s.NoError(s.NewDB().Model(new(model.FdbCluster)).First(&fdbClusterDB).Error)
	fdbClusterExp := model.FdbCluster{
		Model: fdbClusterDB.Model,
	}
	s.Equal(fdbClusterExp, fdbClusterDB)

	var fdbProcessDB model.FdbProcess
	s.NoError(s.NewDB().Model(new(model.FdbProcess)).First(&fdbProcessDB).Error)
	fdbProcessExp := model.FdbProcess{
		Model:  fdbProcessDB.Model,
		Name:   s.Runtime.Services.Fdb.ContainerName,
		NodeID: s.Runtime.LoadNodesMap()[s.step.Node.Name].ID,
	}
	s.Equal(fdbProcessExp, fdbProcessDB)

	s.MockFS.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func (s *runContainerStepSuite) TestRunContainerFailed() {
	s.MockFS.On("MkdirAll", s.dataDir).Return(nil)
	s.MockFS.On("MkdirAll", s.logDir).Return(nil)
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageNameFdb)
	s.NoError(err)
	s.MockDocker.On("Run", &external.RunArgs{
		Image:         img,
		Name:          &s.Cfg.Services.Fdb.ContainerName,
		RestartPolicy: external.ContainerRestartPolicyUnlessStopped,
		HostNetwork:   true,
		Detach:        common.Pointer(true),
		Envs: map[string]string{
			"FDB_CLUSTER_FILE_CONTENTS": "xxxx",
		},
		Volumes: []*external.VolumeArgs{
			{
				Source: s.dataDir,
				Target: "/var/fdb/data",
			},
			{
				Source: s.logDir,
				Target: "/var/fdb/logs",
			},
		},
	}).Return(nil, errors.New("dummy error"))

	s.Error(s.step.Execute(s.Ctx()), "dummy error")

	s.MockFS.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func (s *runContainerStepSuite) TestMkdirDirFailed() {
	s.MockFS.On("MkdirAll", s.dataDir).Return(nil)
	s.MockFS.On("MkdirAll", s.logDir).Return(errors.New("dummy error"))

	s.Error(s.step.Execute(s.Ctx()), "dummy error")

	s.MockFS.AssertExpectations(s.T())
}

func TestInitClusterStepSuite(t *testing.T) {
	suiteRun(t, &initClusterStepSuite{})
}

type initClusterStepSuite struct {
	ttask.StepSuite

	step *initClusterStep
}

func (s *initClusterStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &initClusterStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
	s.Runtime.Store(task.RuntimeFdbClusterFileContentKey, "xxxx")
}

func (s *initClusterStepSuite) TestInit() {
	s.MockDocker.On("Exec", s.Runtime.Services.Fdb.ContainerName,
		"fdbcli", []string{"--exec", "'configure new single ssd'"}).Return("", nil)
	s.MockDocker.On("Exec", s.Runtime.Services.Fdb.ContainerName,
		"fdbcli", []string{"--exec", "'status minimal'"}).
		Return("The database is available.", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockDocker.AssertExpectations(s.T())
}

func (s *initClusterStepSuite) TestInitClusterFailed() {
	s.MockDocker.On("Exec", s.Runtime.Services.Fdb.ContainerName,
		"fdbcli", []string{"--exec", "'configure new single ssd'"}).
		Return(nil, errors.New("dummy error"))

	s.Error(s.step.Execute(s.Ctx()), "dummy error")

	s.MockDocker.AssertExpectations(s.T())
}

func (s *initClusterStepSuite) TestWaitClusterInitializedFailed() {
	s.MockDocker.On("Exec", s.Runtime.Services.Fdb.ContainerName,
		"fdbcli", []string{"--exec", "'configure new single ssd'"}).
		Return("", nil)
	s.MockDocker.On("Exec", s.Runtime.Services.Fdb.ContainerName,
		"fdbcli", []string{"--exec", "'status minimal'"}).
		Return(nil, errors.New("dummy error"))

	s.Error(s.step.Execute(s.Ctx()), "dummy error")

	s.MockDocker.AssertExpectations(s.T())
}

func TestRmContainerStep(t *testing.T) {
	suiteRun(t, &rmContainerStepSuite{})
}

type rmContainerStepSuite struct {
	ttask.StepSuite

	step    *rmContainerStep
	dataDir string
	logDir  string
}

func (s *rmContainerStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &rmContainerStep{}
	s.dataDir = "/root/3fs/fdb/data"
	s.logDir = "/root/3fs/fdb/logs"
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
	s.Runtime.Store(task.RuntimeFdbClusterFileContentKey, "xxxx")
}

func (s *rmContainerStepSuite) TestRmContainerStep() {
	s.MockDocker.On("Rm", s.Cfg.Services.Fdb.ContainerName, true).
		Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.dataDir}).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.logDir}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func (s *rmContainerStepSuite) TestRmContainerFailed() {
	s.MockDocker.On("Rm", s.Cfg.Services.Fdb.ContainerName, true).
		Return("", errors.New("dummy error"))

	s.Error(s.step.Execute(s.Ctx()), "dummy error")

	s.MockDocker.AssertExpectations(s.T())
}

func (s *rmContainerStepSuite) TestRmDirFailed() {
	s.MockDocker.On("Rm", s.Cfg.Services.Fdb.ContainerName, true).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.dataDir}).Return("", errors.New("dummy error"))

	s.Error(s.step.Execute(s.Ctx()), "dummy error")

	s.MockRunner.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}
