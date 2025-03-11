package image

import (
	"fmt"
	"net/url"

	"github.com/open3fs/m3fs/pkg/errors"
)

// Image is the Image info definition
type Image struct {
	Repo string
	Tag  string
}

// GetURL returns full url of image
func (i *Image) GetURL(registry string) (string, error) {
	imgUrl := fmt.Sprintf("%s:%s", i.Repo, i.Tag)
	if registry != "" {
		var err error
		imgUrl, err = url.JoinPath(registry, imgUrl)
		if err != nil {
			return "", errors.Annotatef(err, "get %s url", i.Repo)
		}
	}

	return imgUrl, nil
}

var images = map[string]Image{
	"3fs":        {Repo: "open3fs/3fs", Tag: "20250307"},
	"fdb":        {Repo: "foundationdb/foundationdb", Tag: "20250307"},
	"clickhouse": {Repo: "clickhouse", Tag: "25.1-jammy"},
}

// GetImage returns image full url by component name
func GetImage(registry, compo string) (string, error) {
	i, ok := images[compo]
	if !ok {
		return "", errors.Errorf("image of %s not found", compo)
	}

	return i.GetURL(registry)
}
