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

// MockFS is an mock type for the FSInterface
type MockFS struct {
	mock.Mock
	external.FSInterface
}

// MkdirTemp mock.
func (m *MockFS) MkdirTemp(ctx context.Context, dir, prefix string) (string, error) {
	arg := m.Called(dir, prefix)
	return arg.String(0), arg.Error(1)
}

// MkTempFile mock.
func (m *MockFS) MkTempFile(ctx context.Context, dir string) (string, error) {
	arg := m.Called(dir)
	return arg.String(0), arg.Error(1)
}

// MkdirAll mock.
func (m *MockFS) MkdirAll(ctx context.Context, path string) error {
	return m.Called(path).Error(0)
}

// RemoveAll mock.
func (m *MockFS) RemoveAll(ctx context.Context, path string) error {
	return m.Called(ctx, path).Error(0)
}

// WriteFile mock.
func (m *MockFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	return m.Called(path, data, perm).Error(0)
}

// DownloadFile mock.
func (m *MockFS) DownloadFile(url, dstPath string) error {
	return m.Called(url, dstPath).Error(0)
}

// ReadRemoteFile mock.
func (m *MockFS) ReadRemoteFile(url string) (string, error) {
	arg := m.Called(url)
	return arg.String(0), arg.Error(1)
}

// IsNotExist mock.
func (m *MockFS) IsNotExist(path string) (bool, error) {
	arg := m.Called(path)
	return arg.Bool(0), arg.Error(1)
}

// Sha256sum mock.
func (m *MockFS) Sha256sum(ctx context.Context, path string) (string, error) {
	arg := m.Called(path)
	return arg.String(0), arg.Error(1)
}

// Tar mock.
func (m *MockFS) Tar(srcPaths []string, basePath, dstPath string) error {
	return m.Called(srcPaths, basePath, dstPath).Error(0)
}

// ExtractTar mock.
func (m *MockFS) ExtractTar(ctx context.Context, srcPath, dstDir string) error {
	return m.Called(srcPath, dstDir).Error(0)
}
