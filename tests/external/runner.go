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
	"os"

	"github.com/stretchr/testify/mock"

	"github.com/open3fs/m3fs/pkg/external"
)

// MockRunner is an mock type for the RunnerInterface
type MockRunner struct {
	mock.Mock
	external.RunnerInterface
}

// NonSudoExec mock.
func (m *MockRunner) NonSudoExec(ctx context.Context, cmd string, args ...string) (string, error) {
	arg := m.Called(cmd, args)
	err1 := arg.Error(1)
	if err1 != nil {
		return "", err1
	}
	return arg.String(0), nil
}

// Exec mock.
func (m *MockRunner) Exec(ctx context.Context, cmd string, args ...string) (string, error) {
	arg := m.Called(cmd, args)
	err1 := arg.Error(1)
	if err1 != nil {
		return "", err1
	}
	return arg.String(0), nil
}

// Scp mock.
func (m *MockRunner) Scp(ctx context.Context, local, remote string) error {
	arg := m.Called(local, remote)
	return arg.Error(0)
}

// Stat mock.
func (m *MockRunner) Stat(path string) (os.FileInfo, error) {
	arg := m.Called(path)
	err1 := arg.Error(1)
	if err1 != nil {
		return nil, err1
	}
	return arg.Get(0).(os.FileInfo), nil
}
