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

	"github.com/stretchr/testify/mock"

	"github.com/open3fs/m3fs/pkg/external"
)

// MockDocker is an mock type for the DockerInterface
type MockDocker struct {
	mock.Mock
	external.DockerInterface
}

// Run mock.
func (m *MockDocker) Run(ctx context.Context, args *external.RunArgs) (string, error) {
	arg := m.Called(args)
	err1 := arg.Error(1)
	if err1 != nil {
		return "", err1
	}
	return arg.String(0), nil
}

// Cp mock.
func (m *MockDocker) Cp(ctx context.Context, src, container, containerPath string) error {
	arg := m.Called(src, container, containerPath)
	return arg.Error(0)

}

// Rm mock.
func (m *MockDocker) Rm(ctx context.Context, name string, force bool) (string, error) {
	arg := m.Called(name, force)
	err1 := arg.Error(1)
	if err1 != nil {
		return "", err1
	}
	return arg.String(0), nil
}

// Exec mock.
func (m *MockDocker) Exec(ctx context.Context, container, cmd string, args ...string) (
	string, error) {

	arg := m.Called(container, cmd, args)
	err1 := arg.Error(1)
	if err1 != nil {
		return "", err1
	}
	return arg.String(0), nil
}

// Load mock.
func (m *MockDocker) Load(ctx context.Context, path string) (string, error) {
	arg := m.Called(path)
	return arg.String(0), arg.Error(1)
}

// Tag mock.
func (m *MockDocker) Tag(ctx context.Context, src, dst string) error {
	return m.Called(src, dst).Error(0)
}
