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

package pg

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
	ttask "github.com/open3fs/m3fs/tests/task"
)

var suiteRun = suite.Run

func TestRunContainerStepSuite(t *testing.T) {
	suiteRun(t, &runContainerStepSuite{})
}

type runContainerStepSuite struct {
	ttask.StepSuite

	masterNode config.Node
	slaveNode  config.Node
	step       *runContainerStep
	dataDir    string
	scriptDir  string
}

func (s *runContainerStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.masterNode = config.Node{
		Name: "master",
		Host: "1.1.1.1",
	}
	s.slaveNode = config.Node{
		Name: "slave",
		Host: "1.1.1.2",
	}
	s.Cfg.Nodes = []config.Node{s.masterNode, s.slaveNode}
	s.Cfg.Services.Pg.Username = "pguser"
	s.Cfg.Services.Pg.Password = "pgpassword"
	s.Cfg.Services.Pg.ReadOnlyUsername = "rouser"
	s.Cfg.Services.Pg.ReadOnlyPassword = "ropassword"
	s.step = &runContainerStep{
		masterNodeName:  s.masterNode.Name,
		replicaUser:     "repli",
		replicaPassword: "repipass",
	}
	s.dataDir = "/root/3fs/postgresql/data"
	s.scriptDir = "/root/3fs/postgresql/scripts"
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.masterNode, s.Logger)
}

func (s *runContainerStepSuite) testRunContainerStep(
	env map[string]string, vols []*external.VolumeArgs) {

	s.MockFS.On("MkdirAll", s.dataDir).Return(nil)
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageNamePg)
	s.NoError(err)
	s.MockDocker.On("Run", &external.RunArgs{
		Image:       img,
		Name:        &s.Cfg.Services.Pg.ContainerName,
		HostNetwork: true,
		Detach:      common.Pointer(true),
		Envs:        env,
		Volumes:     vols,
	}).Return("", nil)
	s.MockDocker.On("Exec", "3fs-postgres", "pg_isready",
		[]string{"-U", s.Cfg.Services.Pg.Username}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockFS.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func (s *runContainerStepSuite) TestRunContainerStepWithMasterNode() {
	s.MockFS.On("MkdirAll", s.scriptDir).Return(nil)
	pgCfg := s.Cfg.Services.Pg
	scriptContent := fmt.Sprintf(`-- create database open3fs
CREATE DATABASE open3fs WITH OWNER %[1]s ENCODING 'UTF8' TEMPLATE template0;
-- create replica user on new created database
CREATE USER %[2]s WITH ENCRYPTED PASSWORD '%[3]s';
GRANT REPLICATION ON DATABASE open3fs TO %[2]s;
-- create readonly user on new created database
CREATE USER %[4]s WITH ENCRYPTED PASSWORD '%[5]s';
-- grant readonly privileges to %[4]s on database open3fs;
GRANT CONNECT ON DATABASE open3fs TO %[4]s;
\c open3fs;
GRANT USAGE ON SCHEMA public TO %[4]s;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO %[4]s;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO %[4]s;`,
		pgCfg.Username, s.step.replicaUser, s.step.replicaPassword,
		pgCfg.ReadOnlyUsername, pgCfg.ReadOnlyPassword)
	s.MockFS.On("WriteFile", s.scriptDir+"/init_db.sql", []byte(scriptContent), os.FileMode(0644)).
		Return(nil)

	s.testRunContainerStep(map[string]string{
		"POSTGRESQL_USERNAME":             pgCfg.Username,
		"POSTGRESQL_PASSWORD":             pgCfg.Password,
		"POSTGRESQL_REPLICATION_USER":     s.step.replicaUser,
		"POSTGRESQL_REPLICATION_PASSWORD": s.step.replicaPassword,
		"POSTGRESQL_PORT_NUMBER":          fmt.Sprintf("%d", pgCfg.Port),
	}, []*external.VolumeArgs{
		{
			Source: s.dataDir,
			Target: "/bitnami/postgresql",
		},
		{
			Source: s.scriptDir,
			Target: "/docker-entrypoint-initdb.d",
		},
	})
}

func (s *runContainerStepSuite) TestRunContainerStepWithSlaveNode() {
	s.step.Init(s.Runtime, s.MockEm, s.slaveNode, s.Logger)
	pgCfg := s.Cfg.Services.Pg
	s.testRunContainerStep(map[string]string{
		"POSTGRESQL_USERNAME":             pgCfg.Username,
		"POSTGRESQL_PASSWORD":             pgCfg.Password,
		"POSTGRESQL_REPLICATION_USER":     s.step.replicaUser,
		"POSTGRESQL_REPLICATION_PASSWORD": s.step.replicaPassword,
		"POSTGRESQL_PORT_NUMBER":          fmt.Sprintf("%d", pgCfg.Port),
		"POSTGRESQL_REPLICATION_MODE":     "slave",
		"POSTGRESQL_MASTER_HOST":          s.masterNode.Host,
	}, []*external.VolumeArgs{
		{
			Source: s.dataDir,
			Target: "/bitnami/postgresql",
		},
	})
}

func (s *runContainerStepSuite) TestRunContainerFailed() {
	s.step.Init(s.Runtime, s.MockEm, s.slaveNode, s.Logger)
	s.MockFS.On("MkdirAll", s.dataDir).Return(nil)
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageNamePg)
	s.NoError(err)
	pgCfg := s.Cfg.Services.Pg
	s.MockDocker.On("Run", &external.RunArgs{
		Image:       img,
		Name:        &s.Cfg.Services.Pg.ContainerName,
		HostNetwork: true,
		Detach:      common.Pointer(true),
		Envs: map[string]string{
			"POSTGRESQL_USERNAME":             pgCfg.Username,
			"POSTGRESQL_PASSWORD":             pgCfg.Password,
			"POSTGRESQL_REPLICATION_USER":     s.step.replicaUser,
			"POSTGRESQL_REPLICATION_PASSWORD": s.step.replicaPassword,
			"POSTGRESQL_PORT_NUMBER":          fmt.Sprintf("%d", pgCfg.Port),
			"POSTGRESQL_REPLICATION_MODE":     "slave",
			"POSTGRESQL_MASTER_HOST":          s.masterNode.Host,
		},
		Volumes: []*external.VolumeArgs{
			{
				Source: s.dataDir,
				Target: "/bitnami/postgresql",
			},
		},
	}).Return(nil, errors.New("dummy error"))

	s.Error(s.step.Execute(s.Ctx()), "dummy error")

	s.MockFS.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func (s *runContainerStepSuite) TestMkdirDirFailed() {
	s.MockFS.On("MkdirAll", s.dataDir).Return(nil)
	s.MockFS.On("MkdirAll", s.scriptDir).Return(errors.New("dummy error"))

	s.Error(s.step.Execute(s.Ctx()), "dummy error")

	s.MockFS.AssertExpectations(s.T())
}

func TestRmContainerStepSuite(t *testing.T) {
	suiteRun(t, &rmContainerStepSuite{})
}

type rmContainerStepSuite struct {
	ttask.StepSuite

	step    *rmContainerStep
	workDir string
}

func (s *rmContainerStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &rmContainerStep{}
	s.workDir = "/root/3fs/postgresql"
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *rmContainerStepSuite) TestRmContainerStep() {
	s.MockDocker.On("Rm", s.Cfg.Services.Pg.ContainerName, true).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", "/root/3fs/postgresql"}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func (s *rmContainerStepSuite) TestRmContainerFailed() {
	s.MockDocker.On("Rm", s.Cfg.Services.Pg.ContainerName, true).
		Return("", errors.New("dummy error"))

	s.Error(s.step.Execute(s.Ctx()), "dummy error")

	s.MockDocker.AssertExpectations(s.T())
}

func (s *rmContainerStepSuite) TestRmDirFailed() {
	s.MockDocker.On("Rm", s.Cfg.Services.Pg.ContainerName, true).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.workDir}).Return("", errors.New("dummy error"))

	s.Error(s.step.Execute(s.Ctx()), "dummy error")

	s.MockRunner.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func TestInitResourceModelsStepSuite(t *testing.T) {
	suiteRun(t, &initResourceModelsStepSuite{})
}

type initResourceModelsStepSuite struct {
	ttask.StepSuite

	step *initResourceModelsStep
}

func (s *initResourceModelsStepSuite) SetupSuite() {

	s.StepSuite.SetupSuite()
	s.CreateTables = false
}

func (s *initResourceModelsStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &initResourceModelsStep{}
	connArgs := s.ConnectionArgs()
	s.Cfg.Nodes = []config.Node{{Name: "test", Host: connArgs.Host}}
	s.Cfg.Services.Pg.Nodes = []string{"test"}
	s.Cfg.Services.Pg.Port = connArgs.Port
	s.Cfg.Services.Pg.Username = connArgs.User
	s.Cfg.Services.Pg.Password = connArgs.Password
	s.Cfg.Services.Pg.Database = s.ConnectionArgs().DBName
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *initResourceModelsStepSuite) TestInitResourceModelsStep() {
	s.NoError(s.step.Execute(s.Ctx()))

	db := s.NewDB()
	var clusterDB model.Cluster
	s.NoError(db.First(&clusterDB).Error)
	clusterExp := model.Cluster{
		Model:             clusterDB.Model,
		Name:              s.Cfg.Name,
		NetworkType:       string(s.Cfg.NetworkType),
		DiskType:          string(s.Cfg.Services.Storage.DiskType),
		ReplicationFactor: s.Cfg.Services.Storage.ReplicationFactor,
		DiskNumPerNode:    s.Cfg.Services.Storage.DiskNumPerNode,
	}
	s.Equal(clusterExp, clusterDB)

	var nodeDB model.Node
	s.NoError(db.First(&nodeDB).Error)
	nodeExp := model.Node{
		Model: nodeDB.Model,
		Name:  s.Cfg.Nodes[0].Name,
		Host:  s.Cfg.Nodes[0].Host,
	}
	s.Equal(nodeExp, nodeDB)

	var pgServiceDB model.PgService
	s.NoError(db.First(&pgServiceDB).Error)
	pgServiceExp := model.PgService{
		Model:  pgServiceDB.Model,
		Name:   s.Cfg.Services.Pg.ContainerName,
		NodeID: nodeExp.ID,
	}
	s.Equal(pgServiceExp, pgServiceDB)

	cacheDB, ok := s.step.Runtime.Load(task.RuntimeDbKey)
	s.True(ok)
	s.NoError(model.CloseDB(cacheDB.(*gorm.DB)))

	nodesMap := s.Runtime.LoadNodesMap()
	s.True(ok)
	s.Len(nodesMap, 1)
	nodeCache, ok := nodesMap[nodeExp.Name]
	s.True(ok)
	s.Equal(nodeExp.Name, nodeCache.Name)
	s.Equal(nodeExp.ID, nodeCache.ID)
}
