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

package storage

import (
	"embed"
	"fmt"
	"path"
	"strconv"
	"strings"

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

	// StorageMainAppTomlTmpl is the template content of storage_main_app.toml
	StorageMainAppTomlTmpl []byte
	// StorageMainLauncherTomlTmpl is the template content of storage_main_launcher.toml
	StorageMainLauncherTomlTmpl []byte
	// StorageMainTomlTmpl is the template content of storage_main.toml
	StorageMainTomlTmpl []byte
	// DiskToolScriptTmpl is the template content of disk_tool.sh
	DiskToolScriptTmpl []byte
)

func init() {
	var err error
	StorageMainAppTomlTmpl, err = templatesFs.ReadFile("templates/storage_main_app.toml.tmpl")
	if err != nil {
		panic(err)
	}

	StorageMainLauncherTomlTmpl, err = templatesFs.ReadFile("templates/storage_main_launcher.toml.tmpl")
	if err != nil {
		panic(err)
	}

	StorageMainTomlTmpl, err = templatesFs.ReadFile("templates/storage_main.toml.tmpl")
	if err != nil {
		panic(err)
	}

	DiskToolScriptTmpl, err = templatesFs.ReadFile("templates/disk_tool.sh.tmpl")
	if err != nil {
		panic(err)
	}
}

func makeTargetPaths(diskNum int) string {
	targets := make([]string, diskNum)
	for i := 0; i < diskNum; i++ {
		targets[i] = fmt.Sprintf(`"%s"`,
			path.Join("/mnt", "3fsdata", "data"+strconv.Itoa(i), "3fs"))
	}

	return fmt.Sprintf("[%s]", strings.Join(targets, ","))
}

const (
	// ServiceName is the name of the storage service.
	ServiceName = "storage_main"
	serviceType = "STORAGE"
)

func getServiceWorkDir(workDir string) string {
	return path.Join(workDir, "storage")
}

// CreateStorageServiceTask is a task for creating 3fs storage services.
type CreateStorageServiceTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *CreateStorageServiceTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("CreateStorageServiceTask")
	t.BaseTask.Init(r, logger)

	storage := r.Cfg.Services.Storage
	workDir := getServiceWorkDir(r.WorkDir)
	nodes := make([]config.Node, len(storage.Nodes))
	for i, node := range storage.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: steps.NewGen3FSNodeIDStepFunc(ServiceName, 10001, storage.Nodes),
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewRemoteRunScriptStepFunc(
				workDir,
				"disk_tool.sh",
				DiskToolScriptTmpl,
				map[string]any{
					"SectorSize": t.Runtime.Cfg.Services.Storage.SectorSize,
				},
				[]string{
					workDir,
					strconv.Itoa(storage.DiskNumPerNode),
					string(storage.DiskType),
					"prepare",
				}),
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewPrepare3FSConfigStepFunc(&steps.Prepare3FSConfigStepSetup{
				Service:              ServiceName,
				ServiceWorkDir:       workDir,
				MainAppTomlTmpl:      StorageMainAppTomlTmpl,
				MainLauncherTomlTmpl: StorageMainLauncherTomlTmpl,
				MainTomlTmpl:         StorageMainTomlTmpl,
				RDMAListenPort:       storage.RDMAListenPort,
				TCPListenPort:        storage.TCPListenPort,
				ExtraMainTomlData: map[string]any{
					"TargetPaths": makeTargetPaths(storage.DiskNumPerNode),
				},
			}),
		},
		{
			Nodes: []config.Node{nodes[0]},
			NewStep: steps.NewUpload3FSMainConfigStepFunc(
				config.ImageName3FS,
				storage.ContainerName,
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
					ContainerName:  storage.ContainerName,
					Service:        ServiceName,
					WorkDir:        workDir,
					UseRdmaNetwork: true,
					ExtraVolumes: []*external.VolumeArgs{
						{
							Source: path.Join(workDir, "3fsdata"),
							Target: "/mnt/3fsdata",
						},
					},
					ModelObjFunc: func(s *task.BaseStep) any {
						fsNodeID, _ := s.Runtime.LoadInt(
							steps.GetNodeIDKey(ServiceName, s.Node.Name))
						return &model.StorageService{
							Name:     r.Services.Storage.ContainerName,
							NodeID:   s.GetNodeModelID(),
							FsNodeID: fmt.Sprintf("%d", fsNodeID),
						}
					},
				}),
		},
		{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(createDisksStep) },
		},
	})
}

// DeleteStorageServiceTask is a task for deleting a storage services.
type DeleteStorageServiceTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *DeleteStorageServiceTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("DeleteStorageServiceTask")
	t.BaseTask.Init(r, logger)
	nodes := make([]config.Node, len(r.Cfg.Services.Storage.Nodes))
	for i, node := range r.Cfg.Services.Storage.Nodes {
		nodes[i] = r.Nodes[node]
	}
	storage := r.Services.Storage
	workDir := getServiceWorkDir(r.WorkDir)
	t.SetSteps([]task.StepConfig{
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewRm3FSContainerStepFunc(
				r.Services.Storage.ContainerName,
				ServiceName,
				workDir),
		},
		{
			Nodes:    nodes,
			Parallel: true,
			NewStep: steps.NewRemoteRunScriptStepFunc(
				workDir,
				"disk_tool.sh",
				DiskToolScriptTmpl,
				map[string]any{
					"SectorSize": t.Runtime.Cfg.Services.Storage.SectorSize,
				},
				[]string{
					workDir,
					strconv.Itoa(storage.DiskNumPerNode),
					string(storage.DiskType),
					"clear",
				}),
		},
	})
}
