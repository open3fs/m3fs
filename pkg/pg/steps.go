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
	"bytes"
	"context"
	"embed"
	"fmt"
	"path"
	"text/template"
	"time"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
)

var (
	//go:embed templates/*.tmpl
	templatesFs embed.FS

	// InitDbScriptTmpl is the template content of init_db.sql
	InitDbScriptTmpl []byte
)

func init() {
	var err error
	InitDbScriptTmpl, err = templatesFs.ReadFile("templates/init_db.sql.tmpl")
	if err != nil {
		panic(err)
	}
}

func getServiceWorkDir(workDir string) string {
	return path.Join(workDir, "postgresql")
}

type runContainerStep struct {
	task.BaseStep

	masterNodeName  string
	replicaUser     string
	replicaPassword string
}

func (s *runContainerStep) getContainerEvs() map[string]string {
	pgCfg := s.Runtime.Cfg.Services.Pg
	envs := map[string]string{
		"POSTGRESQL_USERNAME":             pgCfg.Username,
		"POSTGRESQL_PASSWORD":             pgCfg.Password,
		"POSTGRESQL_REPLICATION_USER":     s.replicaUser,
		"POSTGRESQL_REPLICATION_PASSWORD": s.replicaPassword,
		"POSTGRESQL_PORT_NUMBER":          fmt.Sprintf("%d", pgCfg.Port),
	}
	isMaster := s.Node.Name == s.masterNodeName
	if !isMaster {
		envs["POSTGRESQL_REPLICATION_MODE"] = "slave"
		envs["POSTGRESQL_MASTER_HOST"] = s.Runtime.Nodes[s.masterNodeName].Host
	}

	return envs
}

func (s *runContainerStep) genInitDbScript(scriptPath string) error {
	tmpl, err := template.New("init_db").Parse(string(InitDbScriptTmpl))
	if err != nil {
		return errors.Annotatef(err, "parse init_db.sql.tmpl")
	}
	pgCfg := s.Runtime.Cfg.Services.Pg
	data := map[string]any{
		"Database":         "open3fs",
		"User":             pgCfg.Username,
		"ReplicaUsername":  s.replicaUser,
		"ReplicaPassword":  s.replicaPassword,
		"ReadOnlyUsername": pgCfg.ReadOnlyUsername,
		"ReadOnlyPassword": pgCfg.ReadOnlyPassword,
	}
	script := bytes.NewBuffer(nil)
	err = tmpl.Execute(script, data)
	if err != nil {
		return errors.Annotatef(err, "execute init_db.sql.tmpl")
	}
	err = s.Em.FS.WriteFile(scriptPath, script.Bytes(), 0644)
	if err != nil {
		return errors.Annotatef(err, "write %s", scriptPath)
	}

	s.Logger.Infof("Generated init_db script %s", scriptPath)

	return nil
}

func (s *runContainerStep) Execute(ctx context.Context) error {
	workDir := getServiceWorkDir(s.Runtime.WorkDir)
	dataDir := path.Join(workDir, "data")
	err := s.Em.FS.MkdirAll(ctx, dataDir)
	if err != nil {
		return errors.Annotatef(err, "mkdir %s", dataDir)
	}

	vols := []*external.VolumeArgs{
		{
			Source: dataDir,
			Target: "/bitnami/postgresql",
		},
	}
	if s.Node.Name == s.masterNodeName {
		scriptDir := path.Join(workDir, "scripts")
		err = s.Em.FS.MkdirAll(ctx, scriptDir)
		if err != nil {
			return errors.Annotatef(err, "mkdir %s", scriptDir)
		}
		vols = append(vols, &external.VolumeArgs{
			Source: scriptDir,
			Target: "/docker-entrypoint-initdb.d",
		})
		if err = s.genInitDbScript(path.Join(scriptDir, "init_db.sql")); err != nil {
			return errors.Trace(err)
		}
	}

	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageNamePg)
	if err != nil {
		return errors.Trace(err)
	}
	pgCfg := s.Runtime.Services.Pg
	args := &external.RunArgs{
		Image:         img,
		Name:          &pgCfg.ContainerName,
		RestartPolicy: external.ContainerRestartPolicyUnlessStepped,
		HostNetwork:   true,
		Detach:        common.Pointer(true),
		Envs:          s.getContainerEvs(),
		Volumes:       vols,
	}
	_, err = s.Em.Docker.Run(ctx, args)
	if err != nil {
		return errors.Trace(err)
	}

	timer := time.NewTimer(pgCfg.WaitReadyTimeout)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return errors.Errorf("wait postgresql container %s ready timeout", pgCfg.ContainerName)
		default:
			_, err := s.Em.Docker.Exec(ctx, pgCfg.ContainerName, "pg_isready", "-U", pgCfg.Username)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
		}
		break
	}

	s.Logger.Infof("Started postgresql container %s successfully", s.Runtime.Services.Pg.ContainerName)
	return nil
}

type rmContainerStep struct {
	task.BaseStep
}

func (s *rmContainerStep) Execute(ctx context.Context) error {
	containerName := s.Runtime.Services.Pg.ContainerName
	_, err := s.Em.Docker.Rm(ctx, containerName, true)
	if err != nil {
		return errors.Trace(err)
	}
	s.Logger.Infof("Removed postgresql container %s", containerName)

	workDir := getServiceWorkDir(s.Runtime.WorkDir)
	_, err = s.Em.Runner.Exec(ctx, "rm", "-rf", workDir)
	if err != nil {
		return errors.Annotatef(err, "rm %s", workDir)
	}
	s.Logger.Infof("Removed postgresql container work dir %s", workDir)

	return nil
}

type initResourceModelsStep struct {
	task.BaseStep
}

func (s *initResourceModelsStep) Execute(ctx context.Context) error {
	node := s.Runtime.Nodes[s.Runtime.Services.Pg.Nodes[0]]
	pgCfg := s.Runtime.Cfg.Services.Pg
	connArg := &model.ConnectionArgs{
		Host:     node.Host,
		Port:     pgCfg.Port,
		User:     pgCfg.Username,
		Password: pgCfg.Password,
		DBName:   pgCfg.Database,
	}
	db, err := model.NewDB(connArg)
	if err != nil {
		return errors.Annotatef(err, "create db connection to %s", connArg)
	}

	if err = model.SyncTables(db); err != nil {
		return errors.Trace(err)
	}

	cfg := s.Runtime.Cfg
	storCfg := cfg.Services.Storage
	cluster := &model.Cluster{
		Name:              cfg.Name,
		NetworkType:       string(cfg.NetworkType),
		DiskType:          string(storCfg.DiskType),
		ReplicationFactor: storCfg.ReplicationFactor,
		DiskNumPerNode:    storCfg.DiskNumPerNode,
	}
	if err = db.Model(new(model.Cluster)).Create(cluster).Error; err != nil {
		return errors.Annotatef(err, "create cluster")
	}

	nodes := make([]*model.Node, 0, len(s.Runtime.Nodes))
	nodesMap := make(map[string]*model.Node, len(s.Runtime.Nodes))
	for _, node := range s.Runtime.Nodes {
		node := &model.Node{
			Name: node.Name,
			Host: node.Host,
		}
		nodes = append(nodes, node)
		nodesMap[node.Name] = node
	}
	if err = db.Model(new(model.Node)).CreateInBatches(nodes, len(nodes)).Error; err != nil {
		return errors.Annotatef(err, "create nodes")
	}
	s.Runtime.Store(task.RuntimeNodesMapKey, nodesMap)

	for _, nodeName := range cfg.Services.Pg.Nodes {
		var node model.Node
		err = db.Model(new(model.Node)).First(&node, "name = ?", nodeName).Error
		if err != nil {
			return errors.Annotatef(err, "query node %s", nodeName)
		}

		pgService := &model.PgService{
			Name:   cfg.Services.Pg.ContainerName,
			NodeID: node.ID,
		}
		if err = db.Model(new(model.PgService)).Create(pgService).Error; err != nil {
			return errors.Annotatef(err, "create pg server of node %s", nodeName)
		}
	}

	s.Runtime.Store(task.RuntimeDbKey, db)

	return nil
}
