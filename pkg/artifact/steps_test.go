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

package artifact

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/task"
	ttask "github.com/open3fs/m3fs/tests/task"
)

var suiteRun = suite.Run

func TestPrepareTmpDirStep(t *testing.T) {
	suiteRun(t, &prepareTmpDirStepSuite{})
}

type prepareTmpDirStepSuite struct {
	ttask.StepSuite

	step *prepareTmpDirStep
}

func (s *prepareTmpDirStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &prepareTmpDirStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime)
	s.Runtime.Store(task.RuntimeArtifactTmpDirKey, "/tmp/3fs")
}

func (s *prepareTmpDirStepSuite) Test() {
	s.MockLocalFS.On("MkdirAll", "/tmp/3fs").Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockLocalFS.AssertExpectations(s.T())
}

type downloadImageInfo struct {
	imageName  string
	fileName   string
	filePath   string
	fileUrl    string
	fileSumUrl string
}

func newDownloadImageInfo(r *task.Runtime, imageName string) *downloadImageInfo {
	fileName, _ := r.Cfg.Images.GetImageFileName(imageName)
	return &downloadImageInfo{
		imageName:  imageName,
		fileName:   fileName,
		filePath:   fmt.Sprintf("/tmp/3fs/%s", fileName),
		fileUrl:    fmt.Sprintf("https://artifactory.open3fs.com/3fs/%s", fileName),
		fileSumUrl: fmt.Sprintf("https://artifactory.open3fs.com/3fs/%s.sha256sum", fileName),
	}
}

func TestDownloadImagesStep(t *testing.T) {
	suiteRun(t, &downloadImagesStepSuite{})
}

type downloadImagesStepSuite struct {
	ttask.StepSuite

	step   *downloadImagesStep
	images []*downloadImageInfo
}

func (s *downloadImagesStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &downloadImagesStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime)
	s.Runtime.Store(task.RuntimeArtifactTmpDirKey, "/tmp/3fs")
	s.images = []*downloadImageInfo{
		newDownloadImageInfo(s.Runtime, config.ImageNameFdb),
		newDownloadImageInfo(s.Runtime, config.ImageNameClickhouse),
		newDownloadImageInfo(s.Runtime, config.ImageName3FS),
	}
}

func (s *downloadImagesStepSuite) TestWithNotExisted() {
	for _, image := range s.images {
		s.MockLocalFS.On("IsNotExist", image.filePath).Return(true, nil)
		s.MockLocalFS.On("DownloadFile", image.fileUrl, image.filePath).Return(nil)
	}

	s.NoError(s.step.Execute(s.Ctx()))

	filePaths, ok := s.Runtime.Load(task.RuntimeArtifactFilePathsKey)
	s.True(ok)
	expectedFilePaths := []string{}
	for _, image := range s.images {
		expectedFilePaths = append(expectedFilePaths, image.filePath)
	}
	s.Equal(expectedFilePaths, filePaths)

	s.MockLocalFS.AssertExpectations(s.T())
}

func (s *downloadImagesStepSuite) TestWithExisted() {
	for _, image := range s.images {
		s.MockLocalFS.On("IsNotExist", image.filePath).Return(false, nil)
		s.MockLocalFS.On("ReadRemoteFile", image.fileSumUrl).Return(
			fmt.Sprintf("xxxx %s", image.fileName), nil)
		s.MockLocalFS.On("Sha256sum", image.filePath).Return("xxxx", nil)
	}

	s.NoError(s.step.Execute(s.Ctx()))

	filePaths, ok := s.Runtime.Load(task.RuntimeArtifactFilePathsKey)
	s.True(ok)
	expectedFilePaths := []string{}
	for _, image := range s.images {
		expectedFilePaths = append(expectedFilePaths, image.filePath)
	}
	s.Equal(expectedFilePaths, filePaths)

	s.MockLocalFS.AssertExpectations(s.T())
}

func TestTarFilesStep(t *testing.T) {
	suiteRun(t, &tarFilesStepSuite{})
}

type tarFilesStepSuite struct {
	ttask.StepSuite

	step *tarFilesStep
}

func (s *tarFilesStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &tarFilesStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime)
	s.Runtime.Store(task.RuntimeArtifactFilePathsKey,
		[]string{"/tmp/3fs/3fs_20250315_amd64.docker"})
	s.Runtime.Store(task.RuntimeArtifactTmpDirKey, "/tmp/3fs")
	s.Runtime.Store(task.RuntimeArtifactPathKey, "/root/3fs.tar.gz")
}

func (s *tarFilesStepSuite) Test() {
	s.MockLocalFS.On("Tar",
		[]string{"/tmp/3fs/3fs_20250315_amd64.docker"},
		"/tmp/3fs",
		"/root/3fs.tar.gz").
		Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockLocalFS.AssertExpectations(s.T())
}

func TestSha256sumArtifactStep(t *testing.T) {
	suiteRun(t, &sha256sumArtifactStepSuite{})
}

type sha256sumArtifactStepSuite struct {
	ttask.StepSuite

	step *sha256sumArtifactStep
}

func (s *sha256sumArtifactStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &sha256sumArtifactStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{})
	s.Runtime.Store(task.RuntimeArtifactPathKey, "/root/3fs.tar.gz")
}

func (s *sha256sumArtifactStepSuite) Test() {
	s.MockLocalFS.On("Sha256sum", "/root/3fs.tar.gz").Return("xxx", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	sha256sum, ok := s.Runtime.LoadString(task.RuntimeArtifactSha256sumKey)
	s.True(ok)
	s.Equal("xxx", sha256sum)
}

func TestDistributeArtifactStep(t *testing.T) {
	suiteRun(t, &distributeArtifactStepSuite{})
}

type distributeArtifactStepSuite struct {
	ttask.StepSuite

	step *distributeArtifactStep
}

func (s *distributeArtifactStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &distributeArtifactStep{}
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{})
	s.Runtime.Store(task.RuntimeArtifactPathKey, "/root/3fs.tar.gz")
	s.Runtime.Store(task.RuntimeArtifactSha256sumKey, "xxx")
}

func (s *distributeArtifactStepSuite) TestWithExisted() {
	s.MockFS.On("Sha256sum", "/root/3fs/3fs.tar.gz").Return("xxx", nil)
	s.MockFS.On("MkdirTemp", "/root/3fs", "artifact").Return("/root/3fs/artifact-xxx", nil)
	s.MockFS.On("ExtractTar", "/root/3fs/3fs.tar.gz", "/root/3fs/artifact-xxx").Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))

	sha256sum, ok := s.Runtime.LoadString(task.RuntimeArtifactSha256sumKey)
	s.True(ok)
	s.Equal("xxx", sha256sum)
	tempDir, ok := s.Runtime.LoadString(task.RuntimeArtifactTmpDirKey)
	s.True(ok)
	s.Equal("/root/3fs/artifact-xxx", tempDir)

	s.MockFS.AssertExpectations(s.T())
}

func (s *distributeArtifactStepSuite) TestWithNotExisted() {
	s.MockFS.On("Sha256sum", "/root/3fs/3fs.tar.gz").Return("", fmt.Errorf("Dummy error"))
	s.MockRunner.On("Scp", "/root/3fs.tar.gz", "/root/3fs/3fs.tar.gz").Return(nil)
	s.MockFS.On("MkdirTemp", "/root/3fs", "artifact").Return("/root/3fs/artifact-xxx", nil)
	s.MockFS.On("ExtractTar", "/root/3fs/3fs.tar.gz", "/root/3fs/artifact-xxx").Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))

	sha256sum, ok := s.Runtime.LoadString(task.RuntimeArtifactSha256sumKey)
	s.True(ok)
	s.Equal("xxx", sha256sum)

	s.MockFS.AssertExpectations(s.T())
	s.MockRunner.AssertExpectations(s.T())
}
