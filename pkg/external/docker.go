package external

import (
	"bytes"
	"context"
	"fmt"

	"github.com/open3fs/m3fs/pkg/errors"
)

// DockerInterface provides interface about docker.
type DockerInterface interface {
	GetContainer(string) string
	Run(ctx context.Context, args *RunArgs) (out *bytes.Buffer, err error)
}

type dockerExternal struct {
	externalBase
}

func (de *dockerExternal) init(em *Manager) {
	de.externalBase.init(em)
	em.Docker = de
}

func (de *dockerExternal) GetContainer(name string) string {
	// TODO: implement docker.GetContainer
	return ""
}

// RunArgs defines args for docker run command.
type RunArgs struct {
	Image   string
	Name    *string
	Detach  *bool
	Publish []*PublishArgs
	Volumes []*VolumeArgs
}

// PublishArgs defines args for publishing a container port.
type PublishArgs struct {
	HostAddress   *string
	HostPort      int
	ContainerPort int
	Protocol      *string
}

// VolumeArgs defines args for binding a volume.
type VolumeArgs struct {
	Source string
	Target string
}

func (de *dockerExternal) Run(ctx context.Context, args *RunArgs) (out *bytes.Buffer, err error) {
	params := []string{"run"}
	if args.Name != nil {
		params = append(params, "--name", *args.Name)
	}
	if args.Detach != nil && *args.Detach {
		params = append(params, "--detach")
	}
	for _, publishArg := range args.Publish {
		publishInfo := fmt.Sprintf("%d:%d", publishArg.HostPort, publishArg.ContainerPort)
		if publishArg.HostAddress != nil {
			publishInfo = *publishArg.HostAddress + ":" + publishInfo
		}
		if publishArg.Protocol != nil {
			publishInfo = publishInfo + "/" + *publishArg.Protocol
		}
		params = append(params, "-p", publishInfo)
	}
	for _, volumeArg := range args.Volumes {
		params = append(params, "--volume", fmt.Sprintf("%s:%s", volumeArg.Source, volumeArg.Target))
	}
	params = append(params, args.Image)
	out, err = de.run(ctx, "docker", params...)
	return out, errors.Trace(err)
}

func init() {
	registerNewExternalFunc(func() externalInterface {
		return new(dockerExternal)
	})
}
