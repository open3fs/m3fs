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

	"github.com/urfave/cli/v2"

	"github.com/open3fs/m3fs/pkg/artifact"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/task"
)

var artifactCmd = &cli.Command{
	Name:    "artifact",
	Aliases: []string{"a"},
	Usage:   "Manage 3fs artifact",
	Subcommands: []*cli.Command{
		{
			Name:   "export",
			Usage:  "Export a 3fs offline artifact",
			Action: exportArtifact,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "config",
					Aliases:     []string{"c"},
					Usage:       "Path to the cluster configuration file",
					Destination: &configFilePath,
					Required:    true,
				},
				&cli.StringFlag{
					Name:        "tmp-dir",
					Aliases:     []string{"t"},
					Usage:       "Temporary dir used to save downloaded packages(default is /tmp/3fs)",
					Destination: &tmpDir,
					Required:    false,
				},
				&cli.StringFlag{
					Name:        "output",
					Aliases:     []string{"o"},
					Usage:       "Output path",
					Destination: &outputPath,
					Required:    true,
				},
			},
		},
	},
}

func exportArtifact(ctx *cli.Context) error {
	cfg, err := loadClusterConfig()
	if err != nil {
		return errors.Trace(err)
	}
	if tmpDir == "" {
		tmpDir = "/tmp/3fs"
	}

	if _, err := os.Stat(outputPath); err == nil {
		return errors.Errorf("output path %s already exists", outputPath)
	} else if !os.IsNotExist(err) {
		return errors.Trace(err)
	}

	runner := task.NewRunner(cfg, new(artifact.ExportArtifactTask))
	runner.Init()
	if err = runner.Store(task.RuntimeArtifactTmpDirKey, tmpDir); err != nil {
		return errors.Trace(err)
	}
	if err = runner.Store(task.RuntimeArtifactPathKey, outputPath); err != nil {
		return errors.Trace(err)
	}
	if err = runner.Run(ctx.Context); err != nil {
		return errors.Annotate(err, "import artifact")
	}

	return nil
}
