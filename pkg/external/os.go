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
	"time"

	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/utils"
)

// OsInterface provides interface about os.
type OsInterface interface {
	RandomString(length int) string
	Sleep(time.Duration)
}

type osExternal struct {
	externalBase
}

func (oe *osExternal) init(em *Manager, logger log.Interface) {
	oe.externalBase.init(em, logger)
	em.Os = oe
}

func (oe *osExternal) RandomString(length int) string {
	return utils.RandomString(length)
}

func (oe *osExternal) Sleep(dur time.Duration) {
	time.Sleep(dur)
}

func init() {
	registerNewExternalFunc(func() externalInterface {
		return new(osExternal)
	})
}
