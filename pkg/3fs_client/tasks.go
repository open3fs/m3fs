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

package fsclient

import (
	"embed"
	"path"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
	"github.com/open3fs/m3fs/pkg/task/steps"
)

var (
	//go:embed templates/*.tmpl
	templatesFs embed.FS

	// ClientFuseMainLauncherTomlTmpl is the template content of hf3fs_fuse_main_launcher.toml
	ClientFuseMainLauncherTomlTmpl []byte
	// ClientMainTomlTmpl is the template content of hf3fs_fuse_main.toml
	ClientMainTomlTmpl []byte
)

func init() {
	var err error
	ClientFuseMainLauncherTomlTmpl, err = templatesFs.ReadFile("templates/hf3fs_fuse_main_launcher.toml.tmpl")
	if err != nil {
		panic(err)
	}

	ClientMainTomlTmpl, err = templatesFs.ReadFile("templates/hf3fs_fuse_main.toml.tmpl")
	if err != nil {
		panic(err)
	}
}

const (
	// ServiceName is the name of the 3fs client service.
	ServiceName = "hf3fs_fuse_main"
	serviceType = "FUSE"
)

func getServiceWorkDir(workDir string) string {
	return path.Join(workDir, "client")
}

// Create3FSClientServiceTask is a task for creating 3fs client services.
type Create3FSClientServiceTask struct {
	task.BaseTask

	// ClientNodes is the nodes name of new client nodes
	ClientNodes []string
	// DeleteContainerIfExists is a flag to delete the container if it exists.
	DeleteContainerIfExists bool
}

// Init initializes the task.
func (t *Create3FSClientServiceTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("Create3FSClientServiceTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(t.ClientNodes))
	client := r.Cfg.Services.Client
	for i, node := range t.ClientNodes {
		nodes[i] = r.Nodes[node]
	}
	runContainerVolumes := []*external.VolumeArgs{}
	if client.HostMountpoint != "" {
		runContainerVolumes = append(runContainerVolumes, &external.VolumeArgs{
			Source: client.HostMountpoint,
			Target: client.HostMountpoint,
			Rshare: common.Pointer(true),
		})
	}
	workDir := getServiceWorkDir(r.WorkDir)
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: steps.NewGenAdminCliConfigStep(),
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewPrepare3FSConfigStepFunc(&steps.Prepare3FSConfigStepSetup{
				Service:              ServiceName,
				ServiceWorkDir:       workDir,
				MainAppTomlTmpl:      []byte(""),
				MainLauncherTomlTmpl: ClientFuseMainLauncherTomlTmpl,
				MainTomlTmpl:         ClientMainTomlTmpl,
				Extra3FSConfigFilesFunc: func(runtime *task.Runtime) []*steps.Extra3FSConfigFile {
					token, _ := r.LoadString(task.RuntimeUserTokenKey)
					return []*steps.Extra3FSConfigFile{
						{
							FileName: "token.txt",
							Data:     []byte(token),
						},
					}
				},
			},
			),
		},
		{
			Nodes: []config.Node{nodes[0]},
			NewStep: steps.NewUpload3FSMainConfigStepFunc(
				config.ImageName3FS,
				client.ContainerName,
				ServiceName,
				workDir,
				serviceType,
			),
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewRun3FSContainerStepFunc(
				&steps.Run3FSContainerStepSetup{
					ImgName:        config.ImageName3FS,
					ContainerName:  client.ContainerName,
					Service:        ServiceName,
					WorkDir:        workDir,
					ExtraVolumes:   runContainerVolumes,
					UseRdmaNetwork: true,
					DeleteIfExists: t.DeleteContainerIfExists,
					ModelObjFunc: func(s *task.BaseStep) any {
						return &model.FuseClient{
							Name:           r.Services.Client.ContainerName,
							NodeID:         s.GetNodeModelID(),
							HostMountpoint: client.HostMountpoint,
						}
					},
				}),
		},
	})
}

// Delete3FSClientServiceTask is a task for deleting a 3fs client services.
type Delete3FSClientServiceTask struct {
	task.BaseTask

	// ClientNodes is the nodes name of new client nodes
	ClientNodes []string
}

// Init initializes the task.
func (t *Delete3FSClientServiceTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("Delete3FSClientServiceTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(t.ClientNodes))
	for i, node := range t.ClientNodes {
		nodes[i] = r.Nodes[node]
	}
	workDir := getServiceWorkDir(r.WorkDir)
	t.SetSteps([]task.StepConfig{
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewRm3FSContainerStepFunc(
				r.Services.Client.ContainerName,
				ServiceName,
				workDir),
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep:  func() task.Step { return new(umountHostMountponitStep) },
		},
	})
}
