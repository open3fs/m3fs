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

package main

import (
	"os"
	"text/template"

	"github.com/urfave/cli/v2"

	"github.com/open3fs/m3fs/pkg/errors"
)

var (
	clusterName      string
	sampleConfigPath string
)

var configCmd = &cli.Command{
	Name:    "config",
	Aliases: []string{"cfg"},
	Usage:   "Manage 3fs config",
	Subcommands: []*cli.Command{
		{
			Name:   "create",
			Usage:  "Create a sample 3fs config",
			Action: createSampleConfig,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "name",
					Aliases:     []string{"n"},
					Usage:       "3FS cluster name (default: \"open3fs\")",
					Value:       "open3fs",
					Destination: &clusterName,
				},
				&cli.StringFlag{
					Name:        "registry",
					Aliases:     []string{"r"},
					Usage:       "Image registry (default is empty)",
					Destination: &registry,
				},
				&cli.StringFlag{
					Name:        "file",
					Aliases:     []string{"f"},
					Usage:       "Specify a configuration file path (default: \"cluster.yml\")",
					Destination: &sampleConfigPath,
					Value:       "cluster.yml",
				},
			},
		},
	},
}

var sampleConfigTemplate = `name: "{{.name}}"
workDir: "/opt/3fs"
# networkType configure the network type of the cluster, can be one of the following:
# -    IB: use InfiniBand network protocol
# -  RDMA: use RDMA network protocol
# - ERDMA: use aliyun ERDMA as RDMA network protocol
# -   RXE: use Linux rxe kernel module to mock RDMA network protocol
networkType: "RDMA"
nodes:
  - name: node1
    host: "192.168.1.1"
    username: "root"
    password: "password"
  - name: node2
    host: "192.168.1.2"
    username: "root"
    password: "password"
services:
  client:
    nodes: 
      - node1
    hostMountpoint: /mnt/3fs
  storage:
    nodes: 
      - node1
      - node2
    # diskType configure the disk type of the storage node to use, can be one of the following:
    # - nvme: NVMe SSD
    # - dir: use a directory on the filesystem
    diskType: "nvme"
  mgmtd:
    nodes: 
      - node1
  meta:
    nodes: 
      - node1
  monitor:
    nodes:
      - node1
  fdb:
    nodes: 
      - node1
  clickhouse:
    nodes: 
      - node1
    # Database name for Clickhouse
    db: "3fs"
    # User for Clickhouse authentication
    user: "default"
    # Password for Clickhouse authentication
    password: "password"
    # TCP port for Clickhouse
    tcpPort: 8999
  grafana:
    nodes: 
      - node1
    # TCP port for Grafana
    port: 3000
images:
  registry: "{{ .registry }}"
  arch: "amd64"
  3fs:
    # If you want to run on environment not support avx512, add -avx2 to the end of the image tag, e.g. 20250410-avx2.
    repo: "open3fs/3fs"
    tag: "20250410"
  fdb: 
    repo: "open3fs/foundationdb"
    tag: "7.3.63"
  clickhouse:
    repo: "open3fs/clickhouse"
    tag: "25.1-jammy"
  grafana:
    repo: "open3fs/grafana"
    tag: "12.0.0"
`

func createSampleConfig(ctx *cli.Context) error {
	tmpl, err := template.New("sampleConfig").Parse(sampleConfigTemplate)
	if err != nil {
		return errors.Annotate(err, "parse sample config template")
	}
	if clusterName == "" {
		return errors.New("cluster name is required")
	}
	if sampleConfigPath == "" {
		sampleConfigPath = "cluster.yml"
	}

	file, err := os.OpenFile(sampleConfigPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return errors.Annotate(err, "create sample config file")
	}
	err = tmpl.Execute(file, map[string]string{
		"name":     clusterName,
		"registry": registry,
	})
	if err != nil {
		return errors.Annotate(err, "write sample config file")
	}

	return nil
}
