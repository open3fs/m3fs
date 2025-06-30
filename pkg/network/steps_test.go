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
	"fmt"
	"os"
	"path"
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
	s.MockLocalFS.On("RemoveAll", tmpDir).Return(nil)

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

func TestSetRxeStepSuite(t *testing.T) {
	suiteRun(t, &setupRxeStepSuite{})
}

type setupRxeStepSuite struct {
	ttask.StepSuite

	step *setupRxeStep
}

func (s *setupRxeStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &setupRxeStep{}
	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
	}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
}

func (s *setupRxeStepSuite) TestSetup() {
	script := `#!/bin/bash

set -e

function load_rdma_rxe_module(){
    echo "Loading rdma_rxe kernel module..."
    modules=($(ls -1 /sys/module))
    declare -A modules_map
    for module in ${modules[@]};do
        modules_map[$module]="1"
    done
    # if any of those modules is loaded, we don't need to load rdma_rxe
    for module in "mlx4_core" "irdma" "erdma" "rdma_rxe";do
        if [ "${modules_map[$module]}" = "1" ];then
            return
        fi
    done

    modprobe rdma_rxe
}

function create_rdma_rxe_link(){
    for netdev in $(ip -o -4 a | awk '{print $2}' | grep -vw lo | sort -u)
    do
        # skip linux bridge
        if ip -o -d l show $netdev | grep -q bridge_id; then
            continue
        fi
    
        if rdma link | grep -q -w "netdev $netdev"; then
            continue
        fi
    
        echo "Create rdma link for $netdev"
        rxe_name="${netdev}_rxe0"
        rdma link add $rxe_name type rxe netdev $netdev
        if rdma link | grep -q -w "link $rxe_name"; then
            echo "Success to create $rxe_name"
        fi
    done
}

load_rdma_rxe_module
create_rdma_rxe_link`
	scriptPath := path.Join(s.Cfg.WorkDir, "bin", "setup-network")
	service := fmt.Sprintf(`[Unit]
Description=Setup 3fs network
Before=docker.service
 
[Service]
Type=simple
ExecStart=%s
 
[Install]
WantedBy=multi-user.target`, scriptPath)
	s.MockCreateService("setup-network", "setup-3fs-network.service", []byte(script), []byte(service))
	s.MockRunner.On("Exec", scriptPath, []string(nil)).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.AssertCreateService()
	s.MockRunner.AssertExpectations(s.T())
}

func TestSetupErdmaStep(t *testing.T) {
	suiteRun(t, &setupErdmaStepSuite{})
}

type setupErdmaStepSuite struct {
	ttask.StepSuite

	step *setupErdmaStep
}

func (s *setupErdmaStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &setupErdmaStep{}
	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
	}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
}

func (s *setupErdmaStepSuite) TestSetup() {
	script := `#!/bin/bash

set -e

function load_erdma_module(){
    echo "Loading erdma kernel module..."
    need_load=true
    if [ -d /sys/module/erdma ];then
        need_load=false
    fi
    if [ "${need_load}" = "false" ];then
        output=$(cat /sys/module/erdma/parameters/compat_mode)
        if [ "$(echo output|xargs)" = "Y" ];then
            return
        fi
        echo "erdma module not running in compat mode, try to remove it"
        set +e
        modprobe -r erdma
        set -e
    fi
    modprobe erdma compat_mode=1
}

load_erdma_module`
	scriptPath := path.Join(s.Cfg.WorkDir, "bin", "setup-network")
	service := fmt.Sprintf(`[Unit]
Description=Setup 3fs network
Before=docker.service
 
[Service]
Type=simple
ExecStart=%s
 
[Install]
WantedBy=multi-user.target`, scriptPath)
	s.MockCreateService("setup-network", "setup-3fs-network.service", []byte(script), []byte(service))
	s.MockRunner.On("Exec", scriptPath, []string(nil)).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.AssertCreateService()
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
