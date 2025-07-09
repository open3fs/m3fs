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

	"github.com/stretchr/testify/mock"

	"github.com/open3fs/m3fs/pkg/external"
)

// MockOs is an mock type for the OsInterface
type MockOs struct {
	mock.Mock
	external.OsInterface
}

// RandomString mock.
func (m *MockOs) RandomString(length int) string {
	arg := m.Called(length)
	return arg.String(0)
}

// Sleep mock.
func (m *MockOs) Sleep(dur time.Duration) {
	m.Called(dur)
}
