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

package clickhouse

import (
	"fmt"
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

func TestGenClickhouseConfigStep(t *testing.T) {
	suiteRun(t, &genClickhouseConfigStepSuite{})
}

type genClickhouseConfigStepSuite struct {
	ttask.StepSuite

	step *genClickhouseConfigStep
}

func (s *genClickhouseConfigStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &genClickhouseConfigStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *genClickhouseConfigStepSuite) Test() {
	s.MockLocalFS.On("MkdirTemp", os.TempDir(), "3fs-clickhouse").
		Return("/tmp/3fs-clickhouse.xxx", nil)
	s.MockLocalFS.On("WriteFile", "/tmp/3fs-clickhouse.xxx/config.xml",
		mock.AnythingOfType("[]uint8"), os.FileMode(0644)).Return(nil)
	s.MockLocalFS.On("WriteFile", "/tmp/3fs-clickhouse.xxx/3fs-monitor.sql",
		mock.AnythingOfType("[]uint8"), os.FileMode(0644)).Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))

	tmpDirValue, ok := s.Runtime.Load(task.RuntimeClickhouseTmpDirKey)
	s.True(ok)
	tmpDir := tmpDirValue.(string)
	s.Equal("/tmp/3fs-clickhouse.xxx", tmpDir)
}

func TestStartContainerStep(t *testing.T) {
	suiteRun(t, &startContainerStepSuite{})
}

type startContainerStepSuite struct {
	ttask.StepSuite

	step *startContainerStep
}

func (s *startContainerStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &startContainerStep{}
	s.Cfg.Nodes = append(s.Cfg.Nodes, config.Node{
		Name: "test-node",
		Host: "1.1.1.1",
	})
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
	s.Runtime.Store(task.RuntimeClickhouseTmpDirKey, "/tmp/3f-clickhouse.xxx")
}

func (s *startContainerStepSuite) TestStartContainerStep() {
	dataDir := "/root/3fs/clickhouse/data"
	logDir := "/root/3fs/clickhouse/log"
	configDir := "/root/3fs/clickhouse/config.d"
	sqlDir := "/root/3fs/clickhouse/sql"
	s.MockFS.On("MkdirAll", dataDir).Return(nil)
	s.MockFS.On("MkdirAll", logDir).Return(nil)
	s.MockFS.On("MkdirAll", configDir).Return(nil)
	s.MockFS.On("MkdirAll", sqlDir).Return(nil)
	s.MockRunner.On("Scp", "/tmp/3f-clickhouse.xxx/config.xml",
		"/root/3fs/clickhouse/config.d/config.xml").Return(nil)
	s.MockRunner.On("Scp", "/tmp/3f-clickhouse.xxx/3fs-monitor.sql",
		"/root/3fs/clickhouse/sql/3fs-monitor.sql").Return(nil)
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageNameClickhouse)
	s.NoError(err)
	s.MockDocker.On("Run", &external.RunArgs{
		Image:         img,
		Name:          common.Pointer("3fs-clickhouse"),
		HostNetwork:   true,
		Detach:        common.Pointer(true),
		RestartPolicy: external.ContainerRestartPolicyUnlessStepped,
		Envs: map[string]string{
			"CLICKHOUSE_USER":     "default",
			"CLICKHOUSE_PASSWORD": "password",
		},
		Volumes: []*external.VolumeArgs{
			{
				Source: dataDir,
				Target: "/var/lib/clickhouse",
			},
			{
				Source: logDir,
				Target: "/var/log/clickhouse-server",
			},
			{
				Source: configDir,
				Target: "/etc/clickhouse-server/config.d",
			},
			{
				Source: sqlDir,
				Target: "/tmp/sql",
			},
		},
	}).Return("", nil)

	s.NotNil(s.step)
	s.NoError(s.step.Execute(s.Ctx()))

	var chServiceDB model.ChService
	s.NoError(s.NewDB().Model(new(model.ChService)).First(&chServiceDB).Error)
	chServiceExp := model.ChService{
		Model:  chServiceDB.Model,
		Name:   s.Runtime.Services.Clickhouse.ContainerName,
		NodeID: s.Runtime.LoadNodesMap()[s.step.Node.Name].ID,
	}
	s.Equal(chServiceExp, chServiceDB)

	s.MockRunner.AssertExpectations(s.T())
	s.MockFS.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
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
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *initClusterStepSuite) TestInit() {
	s.MockDocker.On("Exec", s.Runtime.Services.Clickhouse.ContainerName,
		"bash", []string{
			"-c",
			fmt.Sprintf(`"clickhouse-client --port %d -n < /tmp/sql/3fs-monitor.sql"`,
				s.Runtime.Services.Clickhouse.TCPPort),
		}).
		Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockDocker.AssertExpectations(s.T())
}

func TestRmContainerStep(t *testing.T) {
	suiteRun(t, &rmContainerStepSuite{})
}

type rmContainerStepSuite struct {
	ttask.StepSuite

	step      *rmContainerStep
	dataDir   string
	logDir    string
	configDir string
	sqlDir    string
}

func (s *rmContainerStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &rmContainerStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
	s.dataDir = "/root/3fs/clickhouse/data"
	s.logDir = "/root/3fs/clickhouse/log"
	s.configDir = "/root/3fs/clickhouse/config.d"
	s.sqlDir = "/root/3fs/clickhouse/sql"
}

func (s *rmContainerStepSuite) TestRmContainerStep() {
	s.MockDocker.On("Rm", s.Cfg.Services.Clickhouse.ContainerName, true).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.dataDir}).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.logDir}).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.configDir}).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.sqlDir}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}
