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
	"bytes"
	"context"
	"embed"
	"fmt"
	"math/rand"
	"net"
	"path"
	"text/template"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/task"
)

var (
	//go:embed templates/*
	templatesFs embed.FS

	// DashboardTmpl is the template content of 3fs grafana dashboard
	DashboardTmpl []byte
	// DashboardProvisionTmpl is the template content of 3fs dashboard provision file
	DashboardProvisionTmpl []byte
	// DatasourceProvisionTmpl is the template content of 3fs datasource provision file
	DatasourceProvisionTmpl []byte
)

func init() {
	var err error
	DashboardTmpl, err = templatesFs.ReadFile("templates/dashboard.json.tmpl")
	if err != nil {
		panic(err)
	}

	DashboardProvisionTmpl, err = templatesFs.ReadFile("templates/dashboard_provision.yaml.tmpl")
	if err != nil {
		panic(err)
	}

	DatasourceProvisionTmpl, err = templatesFs.ReadFile("templates/datasource_provision.yaml.tmpl")
	if err != nil {
		panic(err)
	}
}

func getServiceWorkDir(workDir string) string {
	return path.Join(workDir, "grafana")
}

type genGrafanaYamlStep struct {
	task.BaseStep
}

func (s *genGrafanaYamlStep) genYaml(ctx context.Context, dir, filename string, tmpl []byte,
	tmplData map[string]any) error {

	filepath := path.Join(dir, filename)
	s.Logger.Infof("Generating %s", filename)

	if err := s.Em.FS.MkdirAll(ctx, dir); err != nil {
		return errors.Annotatef(err, "mkdir %s", dir)
	}

	t, err := template.New(filepath).Parse(string(tmpl))
	if err != nil {
		return errors.Annotatef(err, "parse %s template", filename)
	}
	data := new(bytes.Buffer)
	if err := t.Execute(data, tmplData); err != nil {
		return errors.Annotatef(err, "execute %s template", filename)
	}
	if err = s.Em.FS.WriteFile(filepath, data.Bytes(), 0644); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (s *genGrafanaYamlStep) Execute(ctx context.Context) error {
	workdir := getServiceWorkDir(s.Runtime.WorkDir)

	chNodeName := s.Runtime.Services.Clickhouse.Nodes[rand.Intn(len(s.Runtime.Services.Clickhouse.Nodes))]
	chNode := s.Runtime.Nodes[chNodeName]
	err := s.genYaml(ctx, path.Join(workdir, "datasources"), "datasource.yaml",
		DatasourceProvisionTmpl, map[string]any{
			"CH_Database": "3fs",
			"CH_Host":     chNode.Host,
			"CH_Port":     s.Runtime.Services.Clickhouse.TCPPort,
			"CH_Username": s.Runtime.Services.Clickhouse.User,
			"CH_Password": s.Runtime.Services.Clickhouse.Password,
		})
	if err != nil {
		return errors.Annotatef(err, "generate datasource provisioning file")
	}

	err = s.genYaml(ctx, path.Join(workdir, "dashboards"), "dashboard.yaml",
		DashboardProvisionTmpl, nil)
	if err != nil {
		return errors.Annotatef(err, "generate dashboard provisioning file")
	}

	err = s.genYaml(ctx, path.Join(workdir, "dashboards"), "3fs.json",
		DashboardTmpl, nil)
	if err != nil {
		return errors.Annotatef(err, "generate dashboard provisioning file")
	}

	return nil
}

type startContainerStep struct {
	task.BaseStep
}

func (s *startContainerStep) Execute(ctx context.Context) error {
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageNameGrafana)
	if err != nil {
		return errors.Trace(err)
	}
	workdir := getServiceWorkDir(s.Runtime.WorkDir)
	datasourceDir := path.Join(workdir, "datasources")
	dashboardDir := path.Join(workdir, "dashboards")
	args := &external.RunArgs{
		Image:       img,
		Name:        &s.Runtime.Services.Grafana.ContainerName,
		HostNetwork: true,
		Detach:      common.Pointer(true),
		Volumes: []*external.VolumeArgs{
			{
				Source: datasourceDir,
				Target: "/etc/grafana/provisioning/datasources",
			},
			{
				Source: dashboardDir,
				Target: "/etc/grafana/provisioning/dashboards",
			},
		},
	}
	_, err = s.Em.Docker.Run(ctx, args)
	if err != nil {
		return errors.Trace(err)
	}

	endpoint := net.JoinHostPort(s.Node.Host, fmt.Sprintf("%d", s.Runtime.Services.Grafana.Port))
	s.Logger.Infof("Started grafana container %s successfully, service endpoint is http://%s,"+
		" login with username \"admin\" and password \"admin\"",
		s.Runtime.Services.Grafana.ContainerName, endpoint)
	return nil
}

type rmContainerStep struct {
	task.BaseStep
}

func (s *rmContainerStep) Execute(ctx context.Context) error {
	containerName := s.Runtime.Services.Grafana.ContainerName
	s.Logger.Infof("Removing grafana container %s", containerName)
	_, err := s.Em.Docker.Rm(ctx, containerName, true)
	if err != nil {
		return errors.Trace(err)
	}

	s.Logger.Infof("Removed grafana container %s successfully", containerName)
	return nil
}
