package external_test

import (
	"testing"

	"github.com/open3fs/m3fs/pkg/external"
)

func TestDockerRunSuite(t *testing.T) {
	suiteRun(t, new(dockerRunSuite))
}

type dockerRunSuite struct {
	Suite
}

func (s *dockerRunSuite) Test() {
	containerName := "3fs-clickhouse"
	detach := true
	hostAddress := "127.0.0.1"
	protocol := "tcp"
	args := &external.RunArgs{
		Image:  "clickhouse/clickhouse-server:latest",
		Name:   &containerName,
		Detach: &detach,
		Envs: map[string]string{
			"A": "B",
		},
		HostNetwork: true,
		Publish: []*external.PublishArgs{
			{
				HostAddress:   &hostAddress,
				HostPort:      9000,
				ContainerPort: 9000,
				Protocol:      &protocol,
			},
		},
		Volumes: []*external.VolumeArgs{
			{
				Source: "/path/to/data",
				Target: "/clickhouse/data",
			},
		},
	}
	mockCmd := "docker run --name 3fs-clickhouse --detach --network host -e A=B -p 127.0.0.1:9000:9000/tcp " +
		"--volume /path/to/data:/clickhouse/data clickhouse/clickhouse-server:latest"
	s.mc.Mock(mockCmd, "", nil)
	_, err := s.em.Docker.Run(s.Ctx(), args)
	s.NoError(err)
}
