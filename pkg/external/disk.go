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

package external

import (
	"context"
	"encoding/json"

	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/log"
)

// BlockDevice represents a block device on the system, including its
// name, label, size, serial number, and any child devices.
type BlockDevice struct {
	Name     string        `json:"name"`
	Label    string        `json:"label"`
	Size     int64         `json:"size"`
	Serial   string        `json:"serial"`
	Children []BlockDevice `json:"children,omitempty"`
}

// DiskInterface provides interface about disk.
type DiskInterface interface {
	GetNvmeDisks() ([]string, error)
	ListBlockDevices(ctx context.Context) ([]BlockDevice, error)
}

type diskExternal struct {
	externalBase
}

func (de *diskExternal) init(em *Manager, logger log.Interface) {
	de.externalBase.init(em, logger)
	em.Disk = de
}

func (de *diskExternal) GetNvmeDisks() ([]string, error) {
	// TODO: implement GetNvmeDisks
	return nil, nil
}

func (de *diskExternal) ListBlockDevices(ctx context.Context) ([]BlockDevice, error) {
	out, err := de.run(ctx, "lsblk", "-J", "-o", "NAME,LABEL,SIZE,SERIAL", "-b")
	if err != nil {
		return nil, errors.Trace(err)
	}

	var devices map[string][]BlockDevice
	if err := json.Unmarshal([]byte(out), &devices); err != nil {
		return nil, errors.Trace(err)
	}

	return devices["blockdevices"], nil
}

func init() {
	registerNewExternalFunc(func() externalInterface {
		return new(diskExternal)
	})
}
