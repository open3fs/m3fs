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
	"fmt"

	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/log"
)

// Container restart policy defines
const (
	ContainerRestartPolicyDefault       = ""
	ContainerRestartPolicyAlways        = "always"
	ContainerRestartPolicyUnlessStepped = "unless-stopped"
)

// DockerInterface provides interface about docker.
type DockerInterface interface {
	GetContainer(string) string
	Run(ctx context.Context, args *RunArgs) (out string, err error)
	Cp(ctx context.Context, src, container, containerPath string) error
	Rm(ctx context.Context, name string, force bool) (out string, err error)
	Exec(context.Context, string, string, ...string) (out string, err error)
	Load(ctx context.Context, path string) (out string, err error)
	Tag(ctx context.Context, src, dst string) error
}

type dockerExternal struct {
	externalBase
}

func (de *dockerExternal) init(em *Manager, logger log.Interface) {
	de.externalBase.init(em, logger)
	em.Docker = de
}

func (de *dockerExternal) GetContainer(name string) string {
	// TODO: implement docker.GetContainer
	return ""
}

// RunArgs defines args for docker run command.
type RunArgs struct {
	Image         string
	HostNetwork   bool
	Entrypoint    *string
	Rm            *bool
	Command       []string
	Privileged    *bool
	Ulimits       map[string]string
	Name          *string
	Detach        *bool
	Publish       []*PublishArgs
	Volumes       []*VolumeArgs
	Envs          map[string]string
	RestartPolicy string
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
	Rshare *bool
}

func (de *dockerExternal) Run(ctx context.Context, args *RunArgs) (out string, err error) {
	params := []string{"run"}
	if args.Name != nil {
		params = append(params, "--name", *args.Name)
	}
	if args.Detach != nil && *args.Detach {
		params = append(params, "--detach")
	}
	if args.HostNetwork {
		params = append(params, "--network", "host")
	}
	for key, val := range args.Envs {
		params = append(params, "-e", fmt.Sprintf("%s=%s", key, val))
	}
	if args.Entrypoint != nil {
		params = append(params, "--entrypoint", *args.Entrypoint)
	}
	if args.Rm != nil && *args.Rm {
		params = append(params, "--rm")
	}
	if args.Privileged != nil && *args.Privileged {
		params = append(params, "--privileged")
	}
	for key, val := range args.Ulimits {
		params = append(params, "--ulimit", fmt.Sprintf("%s=%s", key, val))
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
		volBind := fmt.Sprintf("%s:%s", volumeArg.Source, volumeArg.Target)
		if volumeArg.Rshare != nil && *volumeArg.Rshare {
			volBind += ":rshared"
		}
		params = append(params, "--volume", volBind)
	}
	if args.RestartPolicy != "" {
		params = append(params, "--restart", args.RestartPolicy)
	}
	params = append(params, args.Image)
	if len(args.Command) > 0 {
		params = append(params, args.Command...)
	}
	out, err = de.run(ctx, "docker", params...)
	return out, errors.Trace(err)
}

func (de *dockerExternal) Cp(ctx context.Context, src, container, containerPath string) error {
	_, err := de.run(ctx, "docker", []string{"cp", src, fmt.Sprintf("%s:%s", container, containerPath)}...)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (de *dockerExternal) Rm(ctx context.Context, name string, force bool) (out string, err error) {
	args := []string{"rm"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, name)
	out, err = de.run(ctx, "docker", args...)
	return out, errors.Trace(err)
}

func (de *dockerExternal) Exec(
	ctx context.Context, container, cmd string, args ...string) (out string, err error) {

	params := []string{"exec", container, cmd}
	params = append(params, args...)
	out, err = de.run(ctx, "docker", params...)
	return out, errors.Trace(err)
}

func (de *dockerExternal) Load(ctx context.Context, path string) (out string, err error) {
	out, err = de.run(ctx, "docker", "load", "-i", path)
	return out, errors.Trace(err)
}

func (de *dockerExternal) Tag(ctx context.Context, src, dst string) error {
	_, err := de.run(ctx, "docker", "tag", src, dst)
	return errors.Trace(err)
}

func init() {
	registerNewExternalFunc(func() externalInterface {
		return new(dockerExternal)
	})
}
