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

package network

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/pkg/config"
	ttask "github.com/open3fs/m3fs/tests/task"
)

var suiteRun = suite.Run

func TestGenIbdev2netdevScriptStep(t *testing.T) {
	suiteRun(t, &genIbdev2netdevScriptStepSuite{})
}

type genIbdev2netdevScriptStepSuite struct {
	ttask.StepSuite

	step *genIbdev2netdevScriptStep
}

func (s *genIbdev2netdevScriptStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &genIbdev2netdevScriptStep{}
	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
	}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
}

func (s *genIbdev2netdevScriptStepSuite) TestGenIbdev2netdevScript() {
	tmpDir := "/tmp/m3fs-prepare-network.123"
	s.MockLocalFS.On("MkdirTemp", "/tmp", "m3fs-prepare-network").Return(tmpDir, nil)
	scriptPath := tmpDir + "/gen-ibdev2netdev"
	s.MockLocalFS.On("WriteFile", scriptPath,
		[]byte(genIbdev2netdevScript), os.FileMode(0755)).Return(nil)

	binDir := s.Cfg.WorkDir + "/bin"
	s.MockFS.On("MkdirAll", binDir).Return(nil)
	remoteGenScriptPath := "/tmp/gen-ibdev2netdev"
	s.MockRunner.On("Scp", scriptPath, remoteGenScriptPath).Return(nil)
	s.MockRunner.On("Exec", "bash", []string{remoteGenScriptPath, binDir}).Return("", nil)
	s.MockLocalFS.On("RemoveAll", s.Ctx(), tmpDir).Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockLocalFS.AssertExpectations(s.T())
	s.MockFS.AssertExpectations(s.T())
	s.MockRunner.AssertExpectations(s.T())
}

func TestInstallRdmaPackage(t *testing.T) {
	suiteRun(t, &installRdmaPackageStepSuite{})
}

type installRdmaPackageStepSuite struct {
	ttask.StepSuite

	step *installRdmaPackageStep
}

func (s *installRdmaPackageStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &installRdmaPackageStep{}
	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
	}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
}

func (s *installRdmaPackageStepSuite) TestInstallRdmaPackage() {
	s.MockRunner.On("Exec", "apt", []string{"install", "-y",
		strings.Join(rdmaPackages, " ")}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
}

func TestLoadRdmaRxeModule(t *testing.T) {
	suiteRun(t, &loadRdmaRxeModuleStepSuite{})
}

type loadRdmaRxeModuleStepSuite struct {
	ttask.StepSuite

	step *loadRdmaRxeModuleStep
}

func (s *loadRdmaRxeModuleStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &loadRdmaRxeModuleStep{}
	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
	}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
}

func (s *loadRdmaRxeModuleStepSuite) TestLoadRdmaRxeModule() {
	lsOutput := `8250  crc_t10dif  hid_ntrig
acpi  crct10dif_pclmul  i2c_piix4 libnvdimm`

	s.MockRunner.On("Exec", "ls", []string{"/sys/module"}).Return(lsOutput, nil)
	s.MockRunner.On("Exec", "modprobe", []string{"rdma_rxe"}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
}

func (s *loadRdmaRxeModuleStepSuite) TestNoNeedLoadRdmaRxeModule() {
	for _, module := range []string{"mlx5_core", "irdma", "erdma", "rdma_rxe"} {
		s.testNoNeedLoadRdmaRxeModule(module)
	}
}

func (s *loadRdmaRxeModuleStepSuite) testNoNeedLoadRdmaRxeModule(module string) {
	lsOutput := `8250 crc_t10dif hid_ntrig` + " " + module

	s.MockRunner.On("Exec", "ls", []string{"/sys/module"}).Return(lsOutput, nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
}

func TestCreateRdmaRxeLinkStep(t *testing.T) {
	suiteRun(t, &createRdmaRxeLinkStepSuite{})
}

type createRdmaRxeLinkStepSuite struct {
	ttask.StepSuite

	step *createRdmaRxeLinkStep
}

func (s *createRdmaRxeLinkStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &createRdmaRxeLinkStep{}
	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
	}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
}

func (s *createRdmaRxeLinkStepSuite) TestCreateRdmaRxeLinkStep() {
	tmpDir := "/tmp/m3fs-prepare-network.123"
	s.MockLocalFS.On("MkdirTemp", "/tmp", "m3fs-prepare-network").Return(tmpDir, nil)
	scriptPath := tmpDir + "/create_rdma_rxe_link"
	s.MockLocalFS.On("WriteFile", scriptPath,
		[]byte(createRdmaLinkScript), os.FileMode(0755)).Return(nil)

	binDir := s.Cfg.WorkDir + "/bin"
	remoteScriptPath := binDir + "/create_rdma_rxe_link"
	s.MockFS.On("MkdirAll", binDir).Return(nil)
	s.MockRunner.On("Scp", scriptPath, remoteScriptPath).Return(nil)
	s.MockRunner.On("Exec", "chmod", []string{"+x", remoteScriptPath}).Return("", nil)
	s.MockRunner.On("Exec", "bash", []string{remoteScriptPath}).Return("", nil)

	s.MockLocalFS.On("RemoveAll", s.Ctx(), tmpDir).Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockLocalFS.AssertExpectations(s.T())
	s.MockFS.AssertExpectations(s.T())
	s.MockRunner.AssertExpectations(s.T())
}

func TestLoadErdmaModuleStep(t *testing.T) {
	suiteRun(t, &loadErdmaModuleStepSuite{})
}

type loadErdmaModuleStepSuite struct {
	ttask.StepSuite

	step *loadErdmaModuleStep
}

func (s *loadErdmaModuleStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &loadErdmaModuleStep{}
	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
	}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
}

func (s *loadErdmaModuleStepSuite) TestLoadErdmaModuleNoAction() {
	s.MockRunner.On("Exec", "ls", []string{"/sys/module/erdma"}).
		Return("coresize  drivers  holders  initsize  initstate notes  parameters", nil)
	s.MockRunner.On("Exec", "cat", []string{"/sys/module/erdma/parameters/compat_mode"}).
		Return("Y", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
}

func (s *loadErdmaModuleStepSuite) TestLoadErdmaModuleWithLoadModule() {
	s.MockRunner.On("Exec", "ls", []string{"/sys/module/erdma"}).
		Return("", errors.New("No such file or directory"))
	s.MockRunner.On("Exec", "modprobe", []string{"erdma", "compat_mode=1"}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
}

func (s *loadErdmaModuleStepSuite) TestLoadErdmaModuleWithReloadModule() {
	s.MockRunner.On("Exec", "ls", []string{"/sys/module/erdma"}).
		Return("coresize  drivers  holders  initsize  initstate notes  parameters", nil)
	s.MockRunner.On("Exec", "cat", []string{"/sys/module/erdma/parameters/compat_mode"}).
		Return("N", nil)
	s.MockRunner.On("Exec", "modprobe", []string{"-r", "erdma"}).Return("", nil)
	s.MockRunner.On("Exec", "modprobe", []string{"erdma", "compat_mode=1"}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
}

func TestDeleteIbdev2netdevScriptStep(t *testing.T) {
	suiteRun(t, &deleteIbdev2netdevScriptStepSuite{})
}

type deleteIbdev2netdevScriptStepSuite struct {
	ttask.StepSuite

	step *deleteIbdev2netdevScriptStep
}

func (s *deleteIbdev2netdevScriptStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &deleteIbdev2netdevScriptStep{}
	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
	}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
}

func (s *deleteIbdev2netdevScriptStepSuite) TestDeleteIbdev2netdevScriptStep() {
	scriptPath := s.Cfg.WorkDir + "/bin/ibdev2netdev"
	s.MockRunner.On("Exec", "rm", []string{"-f", scriptPath}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
}

func TestDeleteRdmaRxeLinkScriptStep(t *testing.T) {
	suiteRun(t, &deleteRdmaRxeLinkScriptStepSuite{})
}

type deleteRdmaRxeLinkScriptStepSuite struct {
	ttask.StepSuite

	step *deleteRdmaRxeLinkScriptStep
}

func (s *deleteRdmaRxeLinkScriptStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &deleteRdmaRxeLinkScriptStep{}
	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
	}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
}

func (s *deleteRdmaRxeLinkScriptStepSuite) TestDeleteRdmaRxeLinkScriptStep() {
	scriptPath := s.Cfg.WorkDir + "/bin/create_rdma_rxe_link"
	s.MockRunner.On("Exec", "rm", []string{"-f", scriptPath}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
}
