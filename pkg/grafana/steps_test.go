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

package grafana

import (
	"os"
	"path"
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

func TestGenGrafanaConfigStepSuite(t *testing.T) {
	suiteRun(t, &genGrafanaConfigStepSuite{})
}

type genGrafanaConfigStepSuite struct {
	ttask.StepSuite

	step *genGrafanaYamlStep
}

func (s *genGrafanaConfigStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &genGrafanaYamlStep{}
	s.Cfg.Nodes = []config.Node{{Name: "name", Host: "test"}}
	s.Cfg.Services.Clickhouse.Nodes = []string{"name"}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *genGrafanaConfigStepSuite) Test() {
	workDir := getServiceWorkDir(s.Runtime.WorkDir)
	s.MockFS.On("MkdirAll", path.Join(workDir, "datasources")).Return(nil)
	s.MockFS.On("WriteFile", path.Join(workDir, "datasources", "datasource.yaml"),
		mock.AnythingOfType("[]uint8"), os.FileMode(0644)).Return(nil)
	s.MockFS.On("MkdirAll", path.Join(workDir, "dashboards")).Return(nil)
	s.MockFS.On("WriteFile", path.Join(workDir, "dashboards", "dashboard.yaml"),
		mock.AnythingOfType("[]uint8"), os.FileMode(0644)).Return(nil)
	s.MockFS.On("WriteFile", path.Join(workDir, "dashboards", "3fs.json"),
		mock.AnythingOfType("[]uint8"), os.FileMode(0644)).Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockFS.AssertExpectations(s.T())
}

func TestStartContainerStepSuite(t *testing.T) {
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
	s.Runtime.Store(task.RuntimeGrafanaTmpDirKey, "/tmp/3f-clickhouse.xxx")
}

func (s *startContainerStepSuite) TestStartContainerStep() {
	workDir := getServiceWorkDir(s.Runtime.WorkDir)
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageNameGrafana)
	s.NoError(err)
	s.MockDocker.On("Run", &external.RunArgs{
		Image:         img,
		Name:          common.Pointer("3fs-grafana"),
		RestartPolicy: external.ContainerRestartPolicyUnlessStepped,
		HostNetwork:   true,
		Detach:        common.Pointer(true),
		Volumes: []*external.VolumeArgs{
			{
				Source: path.Join(workDir, "datasources"),
				Target: "/etc/grafana/provisioning/datasources",
			},
			{
				Source: path.Join(workDir, "dashboards"),
				Target: "/etc/grafana/provisioning/dashboards",
			},
		},
	}).Return("", nil)

	s.NotNil(s.step)
	s.NoError(s.step.Execute(s.Ctx()))

	var grafanaServiceDB model.GrafanaService
	s.NoError(s.NewDB().Model(new(model.GrafanaService)).First(&grafanaServiceDB).Error)
	grafanaServiceExp := model.GrafanaService{
		Model:  grafanaServiceDB.Model,
		Name:   s.Runtime.Services.Grafana.ContainerName,
		NodeID: s.Runtime.LoadNodesMap()[s.step.Node.Name].ID,
	}
	s.Equal(grafanaServiceExp, grafanaServiceDB)

	s.MockDocker.AssertExpectations(s.T())
}
func TestRmContainerStepSuite(t *testing.T) {
	suiteRun(t, &rmContainerStepSuite{})
}

type rmContainerStepSuite struct {
	ttask.StepSuite

	step *rmContainerStep
}

func (s *rmContainerStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &rmContainerStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *rmContainerStepSuite) TestRmContainerStep() {
	s.MockDocker.On("Rm", s.Cfg.Services.Grafana.ContainerName, true).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
}
