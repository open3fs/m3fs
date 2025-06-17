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
	"context"
	"strconv"
	"strings"

	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
	"gorm.io/gorm"
)

type createDisksStep struct {
	task.BaseStep

	db               *gorm.DB
	nodeID           uint
	storageServiceID uint
}

func (s *createDisksStep) Execute(ctx context.Context) error {
	s.db = s.Runtime.LoadDB()
	var node model.Node
	if err := s.db.First(&node, "name = ?", s.Node.Name).Error; err != nil {
		return errors.Trace(err)
	}
	s.nodeID = node.ID
	var storageService model.StorageService
	if err := s.db.First(&storageService, "node_id = ?", s.nodeID).Error; err != nil {
		return errors.Trace(err)
	}
	s.storageServiceID = storageService.ID

	devices, err := s.Em.Disk.ListBlockDevices(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	if err = s.createFsDisks(devices); err != nil {
		return errors.Trace(err)
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

func (s *createDisksStep) createFsDisks(devices []external.BlockDevice) error {
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
			Name:             device.Name,
			NodeID:           s.nodeID,
			StorageServiceID: s.storageServiceID,
			Index:            index,
			SizeByte:         device.Size,
			SerialNum:        device.Serial,
		}
		if err := s.db.Model(disk).Create(disk).Error; err != nil {
			return errors.Trace(err)
		}
		s.Logger.Debugf("Created disk: %s, index: %d, size: %d, serial: %s",
			disk.Name, disk.Index, disk.SizeByte, disk.SerialNum)
	}

	return nil
}
