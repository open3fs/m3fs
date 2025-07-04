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
	"bytes"
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"text/template"

	"gorm.io/gorm"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
)

type createDisksStep struct {
	task.BaseStep

	db            *gorm.DB
	nodeID        uint
	storServiceID uint
}

func (s *createDisksStep) Execute(ctx context.Context) error {
	s.db = s.Runtime.LoadDB()
	var node model.Node
	if err := s.db.First(&node, "name = ?", s.Node.Name).Error; err != nil {
		return errors.Trace(err)
	}
	s.nodeID = node.ID
	var storageService model.StorService
	if err := s.db.First(&storageService, "node_id = ?", s.nodeID).Error; err != nil {
		return errors.Trace(err)
	}
	s.storServiceID = storageService.ID

	if s.Runtime.Cfg.Services.Storage.DiskType == config.DiskTypeDirectory {
		if err := s.createDirDisks(); err != nil {
			return errors.Trace(err)
		}
	} else {
		devices, err := s.Em.Disk.ListBlockDevices(ctx)
		if err != nil {
			return errors.Trace(err)
		}
		if err = s.createFsDisks(devices); err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func (s *createDisksStep) parseDiskIndex(label string) (int, error) {
	indexStr := strings.TrimSpace(strings.TrimPrefix(label, "3fs-data-"))
	index, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		return 0, errors.Errorf("invalid disk label %s: %v", label, err)
	}

	return int(index), nil
}

func (s *createDisksStep) createDirDisks() error {
	storCfg := s.Runtime.Services.Storage
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for i := range storCfg.DiskNumPerNode {
			disk := &model.Disk{
				Name:          path.Join(getServiceWorkDir(s.Runtime.WorkDir), "3fsdata", fmt.Sprintf("data%d", i)),
				NodeID:        s.nodeID,
				StorServiceID: s.storServiceID,
				Index:         i,
				SizeByte:      -1,
				SerialNum:     "",
			}
			if err := tx.Model(disk).Create(disk).Error; err != nil {
				return errors.Trace(err)
			}
			s.Logger.Debugf("Created dir type disk: %s, index: %d", disk.Name, disk.Index)
		}
		return nil
	})
	return errors.Trace(err)
}

func (s *createDisksStep) createFsDisks(devices []external.BlockDevice) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, device := range devices {
			if len(device.Children) > 0 {
				if err := errors.Trace(s.createFsDisks(device.Children)); err != nil {
					return errors.Trace(err)
				}
				continue
			}
			if device.Label == "" || !strings.HasPrefix(device.Label, "3fs-data-") {
				continue
			}

			index, err := s.parseDiskIndex(device.Label)
			if err != nil {
				return errors.Trace(err)
			}

			disk := &model.Disk{
				Name:          device.Name,
				NodeID:        s.nodeID,
				StorServiceID: s.storServiceID,
				Index:         index,
				SizeByte:      device.Size,
				SerialNum:     device.Serial,
			}
			if err := tx.Model(disk).Create(disk).Error; err != nil {
				return errors.Trace(err)
			}
			s.Logger.Debugf("Created disk: %s, index: %d, size: %d, serial: %s",
				disk.Name, disk.Index, disk.SizeByte, disk.SerialNum)
		}
		return nil
	})
	return errors.Trace(err)
}

type setupAutoMountDiskStep struct {
	task.BaseStep
}

func (s *setupAutoMountDiskStep) Execute(ctx context.Context) error {
	scriptTmpl, err := template.New("mount_3fs_disks.script").Parse(string(MountDisksScriptTmpl))
	if err != nil {
		return errors.Trace(err)
	}
	scriptPath := path.Join(s.Runtime.WorkDir, "bin", "mount-3fs-disks")
	scriptBuf := bytes.NewBuffer(nil)
	err = scriptTmpl.Execute(scriptBuf, nil)
	if err != nil {
		return errors.Trace(err)
	}
	serviceTmpl, err := template.New("mount_3fs_disks.service").Parse(string(MountDisksServiceTmpl))
	if err != nil {
		return errors.Trace(err)
	}
	tmpData := map[string]any{
		"ScriptPath": scriptPath,
		"BaseDir":    path.Join(getServiceWorkDir(s.Runtime.WorkDir), "3fsdata"),
	}
	serviceBuf := bytes.NewBuffer(nil)
	if err = serviceTmpl.Execute(serviceBuf, tmpData); err != nil {
		return errors.Trace(err)
	}

	err = s.CreateScriptAndService(ctx, "mount-3fs-disks", "mount-3fs-disks.service",
		scriptBuf.Bytes(), serviceBuf.Bytes())
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

type removeAutoMountDiskServiceStep struct {
	task.BaseStep
}

func (s *removeAutoMountDiskServiceStep) Execute(ctx context.Context) error {
	return errors.Trace(s.DeleteService(ctx, "mount-3fs-disks.service"))
}
