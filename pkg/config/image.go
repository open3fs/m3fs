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

package config

import (
	"fmt"
	"net/url"

	"github.com/open3fs/m3fs/pkg/errors"
)

// defines image names
const (
	ImageNameFdb        = "foundationdb"
	ImageNameClickhouse = "clickhouse"
	ImageName3FS        = "3fs"
	ImageNameGrafana    = "grafana"
)

// Image is component container image config
type Image struct {
	Repo string
	Tag  string
}

// Images contains all component container image configs
type Images struct {
	Registry   string `yaml:"registry"`
	FFFS       Image  `yaml:"3fs"` // 3fs cannot used as struct filed name, so we use fffs instead
	Clickhouse Image  `yaml:"clickhouse"`
	Grafana    Image  `yaml:"grafana"`
	Fdb        Image  `yaml:"fdb"`
}

func (i *Images) getImage(imgName string) (Image, error) {
	switch imgName {
	case ImageNameFdb:
		return i.Fdb, nil
	case ImageName3FS:
		return i.FFFS, nil
	case ImageNameClickhouse:
		return i.Clickhouse, nil
	case ImageNameGrafana:
		return i.Grafana, nil
	default:
		return Image{}, errors.Errorf("invalid image name %s", imgName)
	}
}

// GetImage get image path of target component
func (i *Images) GetImage(imgName string) (string, error) {
	imagePath, err := i.GetImageWithoutRegistry(imgName)
	if err != nil {
		return "", errors.Trace(err)
	}
	if i.Registry != "" {
		var err error
		imagePath, err = url.JoinPath(i.Registry, imagePath)
		if err != nil {
			return "", errors.Annotatef(err, "get image path of %s", imgName)
		}
	}

	return imagePath, nil
}

// GetImageWithoutRegistry get image path without registry
func (i *Images) GetImageWithoutRegistry(imgName string) (string, error) {
	img, err := i.getImage(imgName)
	if err != nil {
		return "", errors.Trace(err)
	}
	return fmt.Sprintf("%s:%s", img.Repo, img.Tag), nil
}

// GetImageFileName gets image file name
func (i Images) GetImageFileName(imgName string) (string, error) {
	img, err := i.getImage(imgName)
	if err != nil {
		return "", errors.Trace(err)
	}
	return fmt.Sprintf("%s_%s_amd64.docker", imgName, img.Tag), nil
}
