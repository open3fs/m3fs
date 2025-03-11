package fdb

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/image"
	"github.com/open3fs/m3fs/pkg/task"
	"github.com/open3fs/m3fs/tests/base"
	texternal "github.com/open3fs/m3fs/tests/external"
)

var suiteRun = suite.Run

type baseSuite struct {
	base.Suite

	cfg        *config.Config
	runtime    *task.Runtime
	em         *external.Manager
	mockOS     *texternal.MockOS
	mockDocker *texternal.MockDocker
}

func (s *baseSuite) SetupTest() {
	s.cfg = new(config.Config)
	s.mockOS = new(texternal.MockOS)
	s.mockDocker = new(texternal.MockDocker)
	s.em = &external.Manager{
		OS:     s.mockOS,
		Docker: s.mockDocker,
	}
}

func (s *baseSuite) setupRuntime() {
	s.runtime = &task.Runtime{Cfg: s.cfg}
	s.runtime.Nodes = make(map[string]config.Node, len(s.cfg.Nodes))
	for _, node := range s.cfg.Nodes {
		s.runtime.Nodes[node.Name] = node
	}
	s.runtime.Services = s.cfg.Services
}

func TestGenClusterFileContentStep(t *testing.T) {
	suiteRun(t, &genClusterFileContentStepSuite{})
}

type genClusterFileContentStepSuite struct {
	baseSuite

	step *genClusterFileContentStep
}

func (s *genClusterFileContentStepSuite) SetupTest() {
	s.baseSuite.SetupTest()

	s.step = &genClusterFileContentStep{}
	s.cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
		{
			Name: "node2",
			Host: "1.1.1.2",
		},
	}
	s.cfg.Services.Fdb.Nodes = []string{"node1", "node2"}
	s.cfg.Services.Fdb.Port = 4500
	s.setupRuntime()
	s.step.Init(s.runtime, s.em)
}

func (s *genClusterFileContentStepSuite) TestGenClusterFileContentStep() {
	s.NoError(s.step.Execute(s.Ctx()))

	contentI, ok := s.runtime.Load("fdb_cluster_file_content")
	s.True(ok)
	s.Equal("node1:node1@1.1.1.1:4500,node2:node2@1.1.1.2:4500", contentI.(string))
}

func TestStartContainerStep(t *testing.T) {
	suiteRun(t, &startContainerStepSuite{})
}

type startContainerStepSuite struct {
	baseSuite

	step *startContainerStep
}

func (s *startContainerStepSuite) SetupTest() {
	s.baseSuite.SetupTest()

	s.step = &startContainerStep{}
	s.cfg.Services.Fdb.DataDir = "/var/fdb"
	s.setupRuntime()
	s.step.Init(s.runtime, s.em)
	s.runtime.Store("fdb_cluster_file_content", "xxxx")
}

func (s *startContainerStepSuite) TestStartContainerStep() {
	dataDir := "/var/fdb/data"
	logDir := "/var/fdb/log"
	s.mockOS.On("Exec", "mkdir", []string{"-p", dataDir}).Return("", nil)
	s.mockOS.On("Exec", "mkdir", []string{"-p", logDir}).Return("", nil)
	img, err := image.GetImage("", "fdb")
	s.NoError(err)
	s.mockDocker.On("Run", &external.RunArgs{
		Image:       img,
		Name:        common.Pointer("fdb"),
		HostNetwork: true,
		Envs: map[string]string{
			"FDB_CLUSTER_FILE_CONTENTS": "xxxx",
		},
		Volumes: []*external.VolumeArgs{
			{
				Source: dataDir,
				Target: "/var/fdb/data",
			},
			{
				Source: logDir,
				Target: "/var/fdb/log",
			},
		},
	}).Return(new(bytes.Buffer), nil)

	s.NotNil(s.step)
	s.NoError(s.step.Execute(s.Ctx()))

	s.mockOS.AssertExpectations(s.T())
	s.mockDocker.AssertExpectations(s.T())
}
