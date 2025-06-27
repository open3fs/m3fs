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
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/log"
)

// FSInterface provides interface about local fs, this is not implemented for remote runner.
type FSInterface interface {
	MkdirTemp(context.Context, string, string) (string, error)
	MkTempFile(context.Context, string) (string, error)
	MkdirAll(context.Context, string) error
	RemoveAll(context.Context, string) error
	ReadFile(context.Context, string) (string, error)
	WriteFile(string, []byte, os.FileMode) error
	DownloadFile(string, string) error
	ReadRemoteFile(string) (string, error)
	IsNotExist(string) (bool, error)
	Sha256sum(context.Context, string) (string, error)
	Tar(srcPaths []string, basePath, dstPath string, needGzip bool) error
	ExtractTar(ctx context.Context, srcPath, dstDir string) error
}

type fsExternal struct {
	externalBase

	returnUnimplemented bool
}

func (fe *fsExternal) init(em *Manager, logger log.Interface) {
	fe.externalBase.init(em, logger)
	em.FS = fe
	if _, ok := em.Runner.(*RemoteRunner); ok {
		fe.returnUnimplemented = true
	}
}

func (fe *fsExternal) MkdirTemp(ctx context.Context, dir, prefix string) (string, error) {
	out, err := fe.run(ctx, "mktemp", "-d", "-p", dir, "-t", prefix+".XXXXXX")
	if err != nil {
		return "", errors.Trace(err)
	}
	dirPaht := strings.TrimSpace(out)
	_, err = fe.run(ctx, "chmod", "0777", dirPaht)
	if err != nil {
		return "", errors.Trace(err)
	}
	return dirPaht, nil
}

func (fe *fsExternal) MkTempFile(ctx context.Context, dir string) (string, error) {
	out, err := fe.run(ctx, "mktemp", "-p", dir)
	if err != nil {
		return "", errors.Trace(err)
	}
	filePath := strings.TrimSpace(out)
	_, err = fe.run(ctx, "chmod", "0777", filePath)
	if err != nil {
		return "", errors.Trace(err)
	}
	return filePath, nil
}

func (fe *fsExternal) MkdirAll(ctx context.Context, dir string) error {
	_, err := fe.run(ctx, "mkdir", "-m", "0777", "-p", dir)
	return errors.Trace(err)
}

func (fe *fsExternal) ReadFile(ctx context.Context, path string) (string, error) {
	out, err := fe.run(ctx, "cat", path)
	if err != nil {
		return "", errors.Trace(err)
	}

	return out, nil
}

func (fe *fsExternal) WriteFile(path string, data []byte, perm os.FileMode) error {
	if fe.returnUnimplemented {
		return errors.New("unimplemented")
	}
	return errors.Trace(os.WriteFile(path, data, perm))
}

func (fe *fsExternal) RemoveAll(ctx context.Context, dir string) error {
	_, err := fe.run(ctx, "rm", "-fr", dir)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (fe *fsExternal) DownloadFile(url, dstPath string) error {
	if fe.returnUnimplemented {
		return errors.New("unimplemented")
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fe.logger.Warnf("Failed to close http client: %v", err)
		}
	}()
	outFile, err := os.Create(dstPath)
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			fe.logger.Warnf("Failed to close file: %v", err)
		}
	}()
	_, err = io.Copy(outFile, resp.Body)
	return err
}

func (fe *fsExternal) ReadRemoteFile(url string) (string, error) {
	if fe.returnUnimplemented {
		return "", errors.New("unimplemented")
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.Trace(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fe.logger.Warnf("Failed to close http client: %v", err)
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Trace(err)
	}

	return string(bodyBytes), nil
}

func (fe *fsExternal) IsNotExist(path string) (bool, error) {
	if _, err := fe.em.Runner.Stat(path); os.IsNotExist(errors.Cause(err)) {
		return true, nil
	}
	return false, nil
}

func (fe *fsExternal) Sha256sum(ctx context.Context, path string) (string, error) {
	out, err := fe.run(ctx, "sha256sum", path)
	if err != nil {
		return "", errors.Trace(err)
	}
	parts := strings.Fields(out)
	if len(parts) < 1 {
		return "", fmt.Errorf("Unexpected output: %s", path)
	}
	return parts[0], nil
}

func (fe *fsExternal) Tar(srcPaths []string, basePath, dstPath string, needGzip bool) error {
	if fe.returnUnimplemented {
		return errors.New("unimplemented")
	}
	outputFile, err := os.Create(dstPath)
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err := outputFile.Close(); err != nil {
			fe.logger.Warnf("Failed to close file: %v", err)
		}
	}()

	var tarWriter *tar.Writer
	if needGzip {
		gzipWriter := gzip.NewWriter(outputFile)
		defer func() {
			if err := gzipWriter.Close(); err != nil {
				fe.logger.Warnf("Failed to close gzip writer: %v", err)
			}
		}()
		tarWriter = tar.NewWriter(gzipWriter)
	} else {
		tarWriter = tar.NewWriter(outputFile)
	}
	defer func() {
		if err := tarWriter.Close(); err != nil {
			fe.logger.Warnf("Failed to close tar writer: %v", err)
		}
	}()

	for _, srcPath := range srcPaths {
		if err := fe.addToTar(tarWriter, srcPath, basePath); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (fe *fsExternal) addToTar(tarWriter *tar.Writer, srcPath, basePath string) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fe.logger.Warnf("Failed to close file: %v", err)
		}
	}()

	relPath, err := filepath.Rel(basePath, srcPath)
	if err != nil {
		return errors.Trace(err)
	}
	relPath = filepath.ToSlash(relPath)

	fileInfo, err := file.Stat()
	if err != nil {
		return errors.Trace(err)
	}
	header, err := tar.FileInfoHeader(fileInfo, filepath.Base(srcPath))
	if err != nil {
		return errors.Trace(err)
	}
	header.Name = relPath
	if err := tarWriter.WriteHeader(header); err != nil {
		return errors.Trace(err)
	}
	if _, err := io.Copy(tarWriter, file); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (fe *fsExternal) ExtractTar(ctx context.Context, srcPath, dstDir string) error {
	_, err := fe.run(ctx, "tar", "-axf", srcPath, "-C", dstDir)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func init() {
	registerNewExternalFunc(func() externalInterface {
		return new(fsExternal)
	})
}
