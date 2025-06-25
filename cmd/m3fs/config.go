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
	"embed"
	"fmt"
	"os"
	"runtime"
	"text/template"

	"github.com/urfave/cli/v2"

	"github.com/open3fs/m3fs/pkg/errors"
)

var (
	clusterName      string
	sampleConfigPath string
	architecture     string

	//go:embed templates/*
	templatesFs embed.FS
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
				&cli.StringFlag{
					Name:        "architecture",
					Aliases:     []string{"arch"},
					Usage:       "Specify the architecture of the cluster (\"amd64\" or \"arm64\")",
					Destination: &architecture,
				},
			},
		},
	},
}

func createSampleConfig(ctx *cli.Context) error {
	if architecture == "" {
		architecture = runtime.GOARCH
	}

	sampleConfigTmpl, err := templatesFs.ReadFile(fmt.Sprintf("templates/config-%s.tmpl", architecture))
	if err != nil {
		return errors.Annotate(err, "read config template")
	}

	tmpl, err := template.New("sampleConfig").Parse(string(sampleConfigTmpl))
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
