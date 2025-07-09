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
	// MountDisksScriptTmpl is the template content of mount_3fs_disks.sh
	MountDisksScriptTmpl []byte
	// MountDisksServiceTmpl is the template content of mount_3fs_disks.service
	MountDisksServiceTmpl []byte
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

	MountDisksScriptTmpl, err = templatesFs.ReadFile("templates/mount_3fs_disks.sh.tmpl")
	if err != nil {
		panic(err)
	}

	MountDisksServiceTmpl, err = templatesFs.ReadFile("templates/mount_3fs_disks.service.tmpl")
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

	DeleteContainerIfExists bool
	// StorageNodes is the nodes name of new storage nodes
	StorageNodes []string
	// BeginNodeID is the begin node id  of new storage nodes
	BeginNodeID int
}

// Init initializes the task.
func (t *CreateStorageServiceTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("CreateStorageServiceTask")
	t.BaseTask.Init(r, logger)

	storage := r.Cfg.Services.Storage
	workDir := getServiceWorkDir(r.WorkDir)
	nodes := make([]config.Node, len(t.StorageNodes))
	for i, node := range t.StorageNodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: steps.NewGenAdminCliConfigStep(),
		},
		{
			Nodes:   []config.Node{t.Runtime.Nodes[t.Runtime.Cfg.Services.Mgmtd.Nodes[0]]},
			NewStep: steps.NewSetupFdbClusterFileContentStepFun(t.Runtime.WorkDir),
		},
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: steps.NewGen3FSNodeIDStepFunc(ServiceName, t.BeginNodeID, t.StorageNodes),
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
					DeleteIfExists: t.DeleteContainerIfExists,
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
						return &model.StorService{
							Name:     r.Services.Storage.ContainerName,
							NodeID:   s.GetNodeModelID(),
							FsNodeID: int64(fsNodeID),
						}
					},
				}),
		},
		{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(setupAutoMountDiskStep) },
		},
		{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(syncNodeDisksStep) },
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
		{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(removeAutoMountDiskServiceStep) },
		},
	})
}

// PrepareChangePlanTask is a task for prepare add storage node change plan.
type PrepareChangePlanTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *PrepareChangePlanTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("PrepareChangePlanTask")
	t.BaseTask.Init(r, logger)
	mgmtNode := r.Nodes[r.Cfg.Services.Mgmtd.Nodes[0]]
	t.SetSteps([]task.StepConfig{
		{
			Nodes: []config.Node{mgmtNode},
			NewStep: func() task.Step {
				return new(prepareChangePlanStep)
			},
		},
	})
}

// RunChangePlanTask is a task for run change plan.
type RunChangePlanTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *RunChangePlanTask) Init(r *task.Runtime, logger log.Interface) {
	t.BaseTask.SetName("RunChangePlanTask")
	t.BaseTask.Init(r, logger)
	mgmtNode := r.Nodes[r.Cfg.Services.Mgmtd.Nodes[0]]
	t.SetSteps([]task.StepConfig{
		{
			Nodes: []config.Node{mgmtNode},
			NewStep: func() task.Step {
				return new(runChangePlanStep)
			},
		},
	})
}
