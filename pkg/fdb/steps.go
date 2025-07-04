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
	"context"
	"fmt"
	"net"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
)

type genClusterFileContentStep struct {
	task.BaseStep
}

func (s *genClusterFileContentStep) Execute(context.Context) error {
	nodes := make([]string, len(s.Runtime.Services.Fdb.Nodes))
	fdb := s.Runtime.Services.Fdb
	for i, fdbNode := range fdb.Nodes {
		for _, node := range s.Runtime.Nodes {
			if node.Name == fdbNode {
				nodes[i] = net.JoinHostPort(node.Host, strconv.Itoa(fdb.Port))
			}
		}
	}

	clusterFileContent := fmt.Sprintf("%s:%s@%s",
		common.RandomString(10), common.RandomString(10), strings.Join(nodes, ","))
	s.Logger.Debugf("fdb cluster file content: %s", clusterFileContent)
	s.Runtime.Store(task.RuntimeFdbClusterFileContentKey, clusterFileContent)
	return nil
}

func getServiceWorkDir(workDir string) string {
	return path.Join(workDir, "fdb")
}

type runContainerStep struct {
	task.BaseStep
}

func (s *runContainerStep) Execute(ctx context.Context) error {
	workDir := getServiceWorkDir(s.Runtime.WorkDir)
	dataDir := path.Join(workDir, "data")
	err := s.Em.FS.MkdirAll(ctx, dataDir)
	if err != nil {
		return errors.Annotatef(err, "mkdir %s", dataDir)
	}
	logDir := path.Join(workDir, "logs")
	err = s.Em.FS.MkdirAll(ctx, logDir)
	if err != nil {
		return errors.Annotatef(err, "mkdir %s", logDir)
	}
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageNameFdb)
	if err != nil {
		return errors.Trace(err)
	}
	clusterContentI, _ := s.Runtime.Load(task.RuntimeFdbClusterFileContentKey)
	clusterContent := clusterContentI.(string)
	args := &external.RunArgs{
		Image:         img,
		Name:          &s.Runtime.Services.Fdb.ContainerName,
		HostNetwork:   true,
		RestartPolicy: external.ContainerRestartPolicyUnlessStopped,
		Detach:        common.Pointer(true),
		Envs: map[string]string{
			"FDB_CLUSTER_FILE_CONTENTS": clusterContent,
		},
		Volumes: []*external.VolumeArgs{
			{
				Source: dataDir,
				Target: "/var/fdb/data",
			},
			{
				Source: logDir,
				Target: "/var/fdb/logs",
			},
		},
	}
	_, err = s.Em.Docker.Run(ctx, args)
	if err != nil {
		return errors.Trace(err)
	}

	db := s.Runtime.LoadDB()
	if s.Node.Name == s.Runtime.Services.Fdb.Nodes[0] {
		if err = db.Create(new(model.FdbCluster)).Error; err != nil {
			return errors.Annotatef(err, "create fdb cluster in db")
		}
	}

	nodes := s.Runtime.LoadNodesMap()
	err = db.Create(&model.FdbProcess{
		Name:   s.Runtime.Services.Fdb.ContainerName,
		NodeID: nodes[s.Node.Name].ID,
	}).Error
	if err != nil {
		return errors.Annotatef(err, "create fdb-process of node %s in db", s.Node.Name)
	}

	s.Logger.Infof("Started fdb container %s successfully", s.Runtime.Services.Fdb.ContainerName)
	return nil
}

type initClusterStep struct {
	task.BaseStep
}

func (s *initClusterStep) Execute(ctx context.Context) error {
	err := s.initCluster(ctx)
	if err != nil {
		return errors.Trace(err)
	}

	return s.waitClusterInitialized(ctx)
}

func (s *initClusterStep) initCluster(ctx context.Context) error {
	s.Logger.Infof("Initializing fdb cluster")
	// TODO: initialize fdb cluster with replication and coordinator setting
	_, err := s.Em.Docker.Exec(ctx, s.Runtime.Services.Fdb.ContainerName,
		"fdbcli", "--exec", "'configure new single ssd'")
	if err != nil {
		return errors.Annotate(err, "initialize fdb cluster")
	}

	return nil
}

func (s *initClusterStep) waitClusterInitialized(ctx context.Context) error {
	s.Logger.Infof("Waiting for fdb cluster initialized")
	tctx, cancel := context.WithTimeout(ctx, s.Runtime.Services.Fdb.WaitClusterTimeout)
	defer cancel()

	for {
		out, err := s.Em.Docker.Exec(tctx, s.Runtime.Services.Fdb.ContainerName,
			"fdbcli", "--exec", "'status minimal'")
		if err != nil {
			return errors.Annotate(err, "wait fdb cluster initialized")
		}
		if strings.Contains(out, "The database is available.") {
			break
		}
		time.Sleep(time.Second)
	}

	s.Logger.Infof("Initialized fdb cluster")
	return nil
}

type rmContainerStep struct {
	task.BaseStep
}

func (s *rmContainerStep) Execute(ctx context.Context) error {
	containerName := s.Runtime.Services.Fdb.ContainerName
	s.Logger.Infof("Removing fdb container %s", containerName)
	_, err := s.Em.Docker.Rm(ctx, containerName, true)
	if err != nil {
		return errors.Trace(err)
	}

	workDir := getServiceWorkDir(s.Runtime.WorkDir)
	dataDir := path.Join(workDir, "data")
	_, err = s.Em.Runner.Exec(ctx, "rm", "-rf", dataDir)
	if err != nil {
		return errors.Annotatef(err, "rm %s", dataDir)
	}
	s.Logger.Infof("Removed fdb container data dir %s", dataDir)

	logDir := path.Join(workDir, "logs")
	_, err = s.Em.Runner.Exec(ctx, "rm", "-rf", logDir)
	if err != nil {
		return errors.Annotatef(err, "rm %s", logDir)
	}
	s.Logger.Infof("Removed fdb container log dir %s", logDir)

	s.Logger.Infof("Removed fdb container %s successfully", containerName)
	return nil
}
