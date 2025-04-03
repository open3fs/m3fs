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
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/errors"
	mlog "github.com/open3fs/m3fs/pkg/log"
)

var (
	debug            bool
	configFilePath   string
	artifactPath     string
	artifactGzip     bool
	outputPath       string
	tmpDir           string
	workDir          string
	registry         string
	clusterDeleteAll bool
	noColorOutput    bool
)

func main() {
	app := &cli.App{
		Name:  "m3fs",
		Usage: "3FS Deploy Tool",
		Before: func(ctx *cli.Context) error {
			level := logrus.InfoLevel
			if debug {
				level = logrus.DebugLevel
			}
			mlog.InitLogger(level)
			return nil
		},
		Commands: []*cli.Command{
			artifactCmd,
			clusterCmd,
			configCmd,
			osCmd,
			tmplCmd,
		},
		Action: func(ctx *cli.Context) error {
			return cli.ShowAppHelp(ctx)
		},
		ExitErrHandler: func(cCtx *cli.Context, err error) {
			if err != nil {
				logrus.Debugf("Command failed stacktrace: %s", errors.StackTrace(err))
			}
			cli.HandleExitCoder(err)
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "Enable debug mode",
				Destination: &debug,
			},
		},
		Version: fmt.Sprintf(`%s
Git SHA: %s
Build At: %s
Go Version: %s
Go OS/Arch: %s/%s`,
			common.Version,
			common.GitSha[:7],
			common.BuildTime,
			runtime.Version(),
			runtime.GOOS,
			runtime.GOARCH),
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
