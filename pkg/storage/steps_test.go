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
	"fmt"
	"path"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	ttask "github.com/open3fs/m3fs/tests/task"
)

var suiteRun = suite.Run

func TestCreateDisksStepSuite(t *testing.T) {
	suiteRun(t, &createDisksStepSuite{})
}

type createDisksStepSuite struct {
	ttask.StepSuite

	step        *createDisksStep
	storService model.StorService
	node        model.Node
}

func (s *createDisksStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.Cfg.Nodes = []config.Node{{Name: "name", Host: "host"}}
	s.Cfg.Services.Storage.Nodes = []string{s.Cfg.Nodes[0].Name}
	s.step = &createDisksStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
	db := s.NewDB()
	s.NoError(db.Model(new(model.Node)).First(&s.node).Error)
	s.storService.NodeID = s.node.ID
	s.NoError(db.Model(new(model.StorService)).Create(&s.storService).Error)
}

func (s *createDisksStepSuite) TestCreateDisks() {
	s.MockDisk.On("ListBlockDevices").Return([]external.BlockDevice{
		{
			Name:   "vda",
			Size:   10737418240,
			Serial: "1234567890",
			Label:  "3fs-data-0",
		},
		{
			Name:   "vdb",
			Size:   10737418240,
			Serial: "1234567891",
			Label:  "3fs-dat",
		},
		{
			Name:   "vdc",
			Size:   10737418240,
			Serial: "1234567892",
			Label:  "3fs-data-1",
			Children: []external.BlockDevice{
				{
					Name:   "vdc1",
					Size:   10737418239,
					Serial: "1234567892",
					Label:  "3fs-data-1",
				},
			},
		},
	}, nil)

	s.NoError(s.step.Execute(s.Ctx()))

	var disks []model.Disk
	s.NoError(s.NewDB().Model(new(model.Disk)).Order("id asc").Find(&disks).Error)
	s.Len(disks, 2)
	disk1Exp := model.Disk{
		Model:         disks[0].Model,
		Name:          "vda",
		NodeID:        s.node.ID,
		StorServiceID: s.storService.ID,
		Index:         0,
		SizeByte:      10737418240,
		SerialNum:     "1234567890",
	}
	s.Equal(disk1Exp, disks[0])
	disk2Exp := model.Disk{
		Model:         disks[1].Model,
		Name:          "vdc1",
		NodeID:        s.node.ID,
		StorServiceID: s.storService.ID,
		Index:         1,
		SizeByte:      10737418239,
		SerialNum:     "1234567892",
	}
	s.Equal(disk2Exp, disks[1])

	s.MockDisk.AssertExpectations(s.T())
}

func TestSetupAutoMountDiskStepSuite(t *testing.T) {
	suiteRun(t, &setupAutoMountDiskStepSuite{})
}

type setupAutoMountDiskStepSuite struct {
	ttask.StepSuite

	step *setupAutoMountDiskStep
}

func (s *setupAutoMountDiskStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = new(setupAutoMountDiskStep)
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *setupAutoMountDiskStepSuite) TestSetup() {
	scriptPath := path.Join(s.Cfg.WorkDir, "bin", "mount-3fs-disks")
	script := `#!/bin/bash
set -e

base_dir=$1

while IFS= read -r line;do
    disk_info=($line)
    dev_name="/dev/${disk_info[0]}"
    if (( ${#disk_info[@]} > 2));then
        continue
    fi
    IFS="-" label_info=(${disk_info[1]}) # label format: 3fs-data-{index}
    mp=${base_dir}/data${label_info[2]}
    echo mounting ${dev_name} to $mp
    mount -t xfs "${dev_name}" "$mp"
done < <(lsblk -o NAME,LABEL,MOUNTPOINT -l | grep -P "3fs-data-\d+" || true)`
	service := fmt.Sprintf(`[Unit]
Description=Mount 3fs disks
Before=docker.service
 
[Service]
Type=simple
ExecStart=%s %s
 
[Install]
WantedBy=multi-user.target`, scriptPath, getServiceWorkDir(s.Cfg.WorkDir)+"/3fsdata")
	s.MockCreateService("mount-3fs-disks", "mount-3fs-disks.service", []byte(script), []byte(service))

	s.NoError(s.step.Execute(s.Ctx()))

	s.AssertCreateService()
}

func TestRemoveAutoMountDiskServiceStepSuite(t *testing.T) {
	suiteRun(t, &removeAutoMountDiskServiceStepSuite{})
}

type removeAutoMountDiskServiceStepSuite struct {
	ttask.StepSuite

	step *removeAutoMountDiskServiceStep
}

func (s *removeAutoMountDiskServiceStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = new(removeAutoMountDiskServiceStep)
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *removeAutoMountDiskServiceStepSuite) TestRemove() {
	s.MockRemoveService("mount-3fs-disks.service", true)

	s.NoError(s.step.Execute(s.Ctx()))

	s.AssertRemoveService()
}

func (s *removeAutoMountDiskServiceStepSuite) TestRemoveWithServiceNotExists() {
	s.MockRemoveService("mount-3fs-disks.service", false)

	s.NoError(s.step.Execute(s.Ctx()))

	s.AssertRemoveService()
}
