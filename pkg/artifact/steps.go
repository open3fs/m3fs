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
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/task"
)

type prepareTmpDirStep struct {
	task.BaseLocalStep
}

func (s *prepareTmpDirStep) Execute(ctx context.Context) error {
	tmpDir, ok := s.Runtime.LoadString(task.RuntimeArtifactTmpDirKey)
	if !ok {
		return errors.Errorf("Failed to get value of %s", task.RuntimeArtifactTmpDirKey)
	}
	if err := s.Runtime.LocalEm.FS.MkdirAll(ctx, tmpDir); err != nil {
		return errors.Trace(err)
	}
	return nil
}

type downloadImagesStep struct {
	task.BaseLocalStep
}

func (s *downloadImagesStep) Execute(ctx context.Context) error {
	imageNames := []string{
		config.ImageNameFdb,
		config.ImageNameClickhouse,
		config.ImageName3FS,
	}
	for _, imageName := range imageNames {
		filePath, err := s.downloadImage(ctx, imageName)
		if err != nil {
			return errors.Trace(err)
		}
		var filePaths []string
		if filePathsValue, ok := s.Runtime.Load(task.RuntimeArtifactFilePathsKey); ok {
			filePaths = filePathsValue.([]string)
		}
		filePaths = append(filePaths, filePath)
		s.Runtime.Store(task.RuntimeArtifactFilePathsKey, filePaths)
	}
	return nil
}

func (s *downloadImagesStep) getUrl(fileName string) string {
	return fmt.Sprintf("https://artifactory.open3fs.com/3fs/%s", fileName)
}

func (s *downloadImagesStep) downloadImage(ctx context.Context, imageName string) (string, error) {
	imageFileName, err := s.Runtime.Cfg.Images.GetImageFileName(imageName)
	if err != nil {
		return "", errors.Trace(err)
	}
	imageUrl := s.getUrl(imageFileName)
	imageSumFileName := fmt.Sprintf("%s.sha256sum", imageFileName)
	imageSumUrl := s.getUrl(imageSumFileName)

	tmpDir, ok := s.Runtime.LoadString(task.RuntimeArtifactTmpDirKey)
	if !ok {
		return "", errors.Errorf("Failed to get tmp dir for artifact")
	}
	dstPath := filepath.Join(tmpDir, imageFileName)
	notExisted, err := s.Runtime.LocalEm.FS.IsNotExist(dstPath)
	if err != nil {
		return "", errors.Trace(err)
	}
	if !notExisted {
		s.Logger.Infof("File of %s image exists", imageName)
		sumContent, err := s.Runtime.LocalEm.FS.ReadRemoteFile(imageSumUrl)
		if err != nil {
			return "", errors.Trace(err)
		}
		expectedSum := strings.Split(sumContent, " ")[0]
		actualSum, err := s.Runtime.LocalEm.FS.Sha256sum(ctx, dstPath)
		if err != nil {
			return "", errors.Trace(err)
		}
		if expectedSum == actualSum {
			s.Logger.Infof("Skip downloading existed %s image", imageName)
			return dstPath, nil
		}
		s.Logger.Infof("Current sha256sum of file %s is %s, expected %s",
			dstPath, actualSum, expectedSum)
	}

	s.Logger.Infof("Downloading %s image from %s", imageName, imageUrl)
	if err := s.Runtime.LocalEm.FS.DownloadFile(imageUrl, dstPath); err != nil {
		return "", errors.Trace(err)
	}
	s.Logger.Infof("Downloaded %s image", imageName)

	return dstPath, nil
}

type tarFilesStep struct {
	task.BaseLocalStep
}

func (s *tarFilesStep) Execute(context.Context) error {
	filePathsValue, ok := s.Runtime.Load(task.RuntimeArtifactFilePathsKey)
	if !ok {
		return errors.Errorf("Failed to get value of %s", task.RuntimeArtifactFilePathsKey)
	}
	filePaths := filePathsValue.([]string)
	dstPath, ok := s.Runtime.LoadString(task.RuntimeArtifactPathKey)
	if !ok {
		return errors.Errorf("Failed to get value of %s", task.RuntimeArtifactPathKey)
	}
	tmpDir, ok := s.Runtime.LoadString(task.RuntimeArtifactTmpDirKey)
	if !ok {
		return errors.Errorf("Failed to get value of %s", task.RuntimeArtifactTmpDirKey)
	}
	needGzip, ok := s.Runtime.LoadBool(task.RuntimeArtifactGzipKey)
	if !ok {
		return errors.Errorf("Failed to get value of %s", task.RuntimeArtifactGzipKey)
	}

	s.Logger.Infof("Generating tar files %s", dstPath)
	if err := s.Runtime.LocalEm.FS.Tar(filePaths, tmpDir, dstPath, needGzip); err != nil {
		return errors.Trace(err)
	}
	s.Logger.Infof("Generated tar files %s", dstPath)
	return nil
}

type sha256sumArtifactStep struct {
	task.BaseStep
}

func (s *sha256sumArtifactStep) Execute(ctx context.Context) error {
	srcPath, ok := s.Runtime.LoadString(task.RuntimeArtifactPathKey)
	if !ok {
		return errors.Errorf("Failed to get value of %s", task.RuntimeArtifactPathKey)
	}
	s.Logger.Infof("Generating SHA256 checksum of artifact %s", srcPath)
	localSum, err := s.Runtime.LocalEm.FS.Sha256sum(ctx, srcPath)
	if err != nil {
		return errors.Trace(err)
	}
	s.Runtime.Store(task.RuntimeArtifactSha256sumKey, localSum)
	s.Logger.Infof("SHA256 checksum of artifact %s is %s", srcPath, localSum)
	return nil
}

type distributeArtifactStep struct {
	task.BaseStep
}

func (s *distributeArtifactStep) Execute(ctx context.Context) error {
	localSum, ok := s.Runtime.LoadString(task.RuntimeArtifactSha256sumKey)
	if !ok {
		return errors.Errorf("Failed to get value of %s", task.RuntimeArtifactSha256sumKey)
	}

	needCopy := true
	dstPath := filepath.Join(s.Runtime.WorkDir, "3fs.tar.gz")
	if remoteSum, err := s.Em.FS.Sha256sum(ctx, dstPath); err == nil {
		needCopy = remoteSum != localSum
	}
	if needCopy {
		s.Logger.Infof("Copying the artifact to %s", s.Node.Name)
		srcPath, ok := s.Runtime.LoadString(task.RuntimeArtifactPathKey)
		if !ok {
			return errors.Errorf("Failed to get value of %s", task.RuntimeArtifactPathKey)
		}
		if err := s.Em.FS.MkdirAll(ctx, filepath.Dir(dstPath)); err != nil {
			return errors.Trace(err)
		}
		if err := s.Em.Runner.Scp(ctx, srcPath, dstPath); err != nil {
			return errors.Trace(err)
		}
	} else {
		s.Logger.Infof("Skip copying existed artifact to %s", s.Node.Name)
	}

	return nil
}

type importArtifactStep struct {
	task.BaseStep
}

func (s *importArtifactStep) Execute(ctx context.Context) error {
	tempDir, err := s.Em.FS.MkdirTemp(ctx, s.Runtime.WorkDir, "artifact")
	if err != nil {
		return errors.Trace(err)
	}
	s.Runtime.Store(s.GetNodeKey(task.RuntimeArtifactTmpDirKey), tempDir)
	pkgPath := filepath.Join(s.Runtime.WorkDir, "3fs.tar.gz")
	s.Logger.Infof("Extracting the artifact to %s on %s", tempDir, s.Node.Name)
	if err = s.Em.FS.ExtractTar(ctx, pkgPath, tempDir); err != nil {
		return errors.Trace(err)
	}

	imageNames := []string{
		config.ImageNameFdb,
		config.ImageNameClickhouse,
		config.ImageName3FS,
	}
	for _, imageName := range imageNames {
		err := s.loadImage(ctx, imageName, tempDir)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (s *importArtifactStep) loadImage(ctx context.Context, imageName, tempDir string) error {
	imageFileName, err := s.Runtime.Cfg.Images.GetImageFileName(imageName)
	if err != nil {
		return errors.Trace(err)
	}
	imageFilePath := filepath.Join(tempDir, imageFileName)
	s.Logger.Infof("Loading image %s on %s", imageName, s.Node.Name)
	if _, err = s.Em.Docker.Load(ctx, imageFilePath); err != nil {
		return errors.Trace(err)
	}
	if s.Runtime.Cfg.Images.Registry != "" {
		imageWithRegistry, err := s.Runtime.Cfg.Images.GetImage(imageName)
		if err != nil {
			return errors.Trace(err)
		}
		imageWithoutRegistry, err := s.Runtime.Cfg.Images.GetImageWithoutRegistry(imageName)
		if err != nil {
			return errors.Trace(err)
		}
		if err = s.Em.Docker.Tag(ctx, imageWithoutRegistry, imageWithRegistry); err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

type removeArtifactStep struct {
	task.BaseStep
}

func (s *removeArtifactStep) Execute(ctx context.Context) error {
	tempDir, ok := s.Runtime.LoadString(s.GetNodeKey(task.RuntimeArtifactTmpDirKey))
	if !ok {
		return errors.Errorf("Failed to get value of %s",
			s.GetNodeKey(task.RuntimeArtifactTmpDirKey))
	}
	_, err := s.Em.Runner.Exec(ctx, "rm", "-rf", tempDir)
	if err != nil {
		return errors.Annotatef(err, "rm %s", tempDir)
	}
	s.Logger.Infof("Removed temp dir %s", tempDir)

	return nil
}
