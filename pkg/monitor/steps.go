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
	"bytes"
	"context"
	"embed"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
)

var (
	//go:embed templates/*
	templatesFs embed.FS

	// MonitorCollectorMainTmpl is the template content of monitor_collector_main.toml
	MonitorCollectorMainTmpl []byte
)

func init() {
	var err error
	MonitorCollectorMainTmpl, err = templatesFs.ReadFile("templates/monitor_collector_main.tmpl")
	if err != nil {
		panic(err)
	}
}

func getServiceWorkDir(workDir string) string {
	return path.Join(workDir, "monitor")
}

type genMonitorConfigStep struct {
	task.BaseStep
}

func (s *genMonitorConfigStep) Execute(ctx context.Context) error {
	tempDir, err := s.Runtime.LocalEm.FS.MkdirTemp(ctx, os.TempDir(), "3fs-monitor")
	if err != nil {
		return errors.Trace(err)
	}
	s.Runtime.Store(task.RuntimeMonitorTmpDirKey, tempDir)

	fileName := "monitor_collector_main.toml"
	tmpl, err := template.New(fileName).Parse(string(MonitorCollectorMainTmpl))
	if err != nil {
		return errors.Annotate(err, "parse monitor_collector_main.toml template")
	}
	var clickhouseHost string
	for _, clickhouseNode := range s.Runtime.Services.Clickhouse.Nodes {
		for _, node := range s.Runtime.Nodes {
			if node.Name == clickhouseNode {
				clickhouseHost = node.Host
			}
		}
	}
	data := new(bytes.Buffer)
	err = tmpl.Execute(data, map[string]string{
		"Port":               strconv.Itoa(s.Runtime.Services.Monitor.Port),
		"ClickhouseDb":       s.Runtime.Services.Clickhouse.Db,
		"ClickhouseHost":     clickhouseHost,
		"ClickhousePassword": s.Runtime.Services.Clickhouse.Password,
		"ClickhousePort":     strconv.Itoa(s.Runtime.Services.Clickhouse.TCPPort),
		"ClickhouseUser":     s.Runtime.Services.Clickhouse.User,
	})
	if err != nil {
		return errors.Annotate(err, "write monitor_collector_main.toml")
	}
	configPath := filepath.Join(tempDir, fileName)
	if err = s.Runtime.LocalEm.FS.WriteFile(configPath, data.Bytes(), 0644); err != nil {
		return errors.Trace(err)
	}

	return nil
}

type runContainerStep struct {
	task.BaseStep
}

func (s *runContainerStep) Execute(ctx context.Context) error {
	workDir := getServiceWorkDir(s.Runtime.WorkDir)
	etcDir := path.Join(workDir, "etc")
	err := s.Em.FS.MkdirAll(ctx, etcDir)
	if err != nil {
		return errors.Annotatef(err, "mkdir %s", etcDir)
	}
	localConfigDir, _ := s.Runtime.Load(task.RuntimeMonitorTmpDirKey)
	localConfigFile := path.Join(localConfigDir.(string), "monitor_collector_main.toml")
	remoteConfigFile := path.Join(etcDir, "monitor_collector_main.toml")
	if err := s.Em.Runner.Scp(ctx, localConfigFile, remoteConfigFile); err != nil {
		return errors.Annotatef(err, "scp monitor_collector_main.toml")
	}
	logDir := path.Join(workDir, "log")
	err = s.Em.FS.MkdirAll(ctx, logDir)
	if err != nil {
		return errors.Annotatef(err, "mkdir %s", logDir)
	}

	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageName3FS)
	if err != nil {
		return errors.Trace(err)
	}
	args := &external.RunArgs{
		Image:         img,
		Name:          &s.Runtime.Services.Monitor.ContainerName,
		RestartPolicy: external.ContainerRestartPolicyUnlessStepped,
		HostNetwork:   true,
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
	if err := s.GetErdmaSoPath(ctx); err != nil {
		return errors.Trace(err)
	}
	args.Volumes = append(args.Volumes, s.GetRdmaVolumes()...)
	_, err = s.Em.Docker.Run(ctx, args)
	if err != nil {
		return errors.Trace(err)
	}

	nodes := s.Runtime.LoadNodesMap()
	db := s.Runtime.LoadDB()
	if err = db.Create(&model.MonService{
		Name:   s.Runtime.Services.Monitor.ContainerName,
		NodeID: nodes[s.Node.Name].ID,
	}).Error; err != nil {
		return errors.Annotatef(err, "create monitor service in db")
	}

	s.Logger.Infof("Started monitor container %s successfully",
		s.Runtime.Services.Monitor.ContainerName)
	return nil
}

type rmContainerStep struct {
	task.BaseStep
}

func (s *rmContainerStep) Execute(ctx context.Context) error {
	containerName := s.Runtime.Services.Monitor.ContainerName
	s.Logger.Infof("Removing monitor container %s", containerName)
	_, err := s.Em.Docker.Rm(ctx, containerName, true)
	if err != nil {
		return errors.Trace(err)
	}
	workDir := getServiceWorkDir(s.Runtime.WorkDir)
	etcDir := path.Join(workDir, "etc")
	_, err = s.Em.Runner.Exec(ctx, "rm", "-rf", etcDir)
	if err != nil {
		return errors.Annotatef(err, "rm %s", etcDir)
	}
	s.Logger.Infof("Removed monitor container etc dir %s", etcDir)

	logDir := path.Join(workDir, "log")
	_, err = s.Em.Runner.Exec(ctx, "rm", "-rf", logDir)
	if err != nil {
		return errors.Annotatef(err, "rm %s", logDir)
	}
	s.Logger.Infof("Removed monitor container log dir %s", logDir)

	s.Logger.Infof("Removed monitor container %s successfully", containerName)
	return nil
}
