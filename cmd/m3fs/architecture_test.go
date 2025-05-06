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

package main

import (
	"testing"

	"github.com/open3fs/m3fs/pkg/config"
)

func TestArchDiagramSuite(t *testing.T) {
	suiteRun(t, &archDiagramSuite{})
}

type archDiagramSuite struct {
	Suite
	cfg       *config.Config
	generator *ArchDiagram
}

func (s *archDiagramSuite) SetupTest() {
	s.Suite.SetupTest()

	s.cfg = s.newTestConfig()
	generator, err := NewArchDiagram(s.cfg)
	s.NoError(err)
	s.generator = generator
}

func (s *archDiagramSuite) newTestConfig() *config.Config {
	return &config.Config{
		Name:        "test-cluster",
		NetworkType: "Ethernet",
		Nodes: []config.Node{
			{Name: "192.168.1.1", Host: "192.168.1.1"},
			{Name: "192.168.1.2", Host: "192.168.1.2"},
			{Name: "192.168.1.3", Host: "192.168.1.3"},
			{Name: "192.168.1.4", Host: "192.168.1.4"},
		},
		Services: config.Services{
			Mgmtd: config.Mgmtd{
				Nodes: []string{"192.168.1.1"},
			},
			Meta: config.Meta{
				Nodes: []string{"192.168.1.1", "192.168.1.2"},
			},
			Storage: config.Storage{
				Nodes: []string{"192.168.1.2", "192.168.1.3"},
			},
			Client: config.Client{
				Nodes:          []string{"192.168.1.3", "192.168.1.4"},
				HostMountpoint: "/mnt/m3fs",
			},
			Fdb: config.Fdb{
				Nodes: []string{"192.168.1.1"},
			},
			Clickhouse: config.Clickhouse{
				Nodes: []string{"192.168.1.2"},
			},
			Monitor: config.Monitor{
				Nodes: []string{"192.168.1.3"},
			},
		},
	}
}

func (s *archDiagramSuite) TestArchDiagram() {
	diagram := s.generator.Generate()
	s.True(s.generator.renderer.ColorEnabled, "Colors should be enabled by default")

	s.NotEmpty(diagram, "Generated diagram should not be empty")
	s.Contains(diagram, "Cluster: test-cluster", "Diagram should contain cluster name")

	// Check node sections
	s.Contains(diagram, "CLIENT NODES", "Diagram should have CLIENT NODES section")
	s.Contains(diagram, "STORAGE NODES", "Diagram should have STORAGE NODES section")

	// Check IP addresses are present
	s.Contains(diagram, "192.168.1.1", "Diagram should show 192.168.1.1")
	s.Contains(diagram, "192.168.1.2", "Diagram should show 192.168.1.2")
	s.Contains(diagram, "192.168.1.3", "Diagram should show 192.168.1.3")
	s.Contains(diagram, "192.168.1.4", "Diagram should show 192.168.1.4")

	// Check service labels are present
	s.Contains(diagram, "[mgmtd]", "Diagram should show mgmtd service")
	s.Contains(diagram, "[meta]", "Diagram should show meta service")
	s.Contains(diagram, "[storage]", "Diagram should show storage service")
	s.Contains(diagram, "[hf3fs_fuse]", "Diagram should show hf3fs_fuse service")
	s.Contains(diagram, "[foundationdb]", "Diagram should show foundationdb service")
	s.Contains(diagram, "[clickhouse]", "Diagram should show clickhouse service")
	s.Contains(diagram, "[monitor]", "Diagram should show monitor service")
}

func (s *archDiagramSuite) TestNoColorOption() {
	s.generator.SetColorEnabled(false)
	s.False(s.generator.renderer.ColorEnabled, "Colors should be disabled after setting ColorEnabled to false")

	diagram := s.generator.Generate()

	// Check if the output does not contain color codes
	s.NotContains(diagram, "\033[", "Diagram should not contain color codes when colors are disabled")

	// Check if the diagram content is still complete
	s.Contains(diagram, "Cluster: test-cluster", "Diagram should still contain cluster name")
	s.Contains(diagram, "CLIENT NODES", "Diagram should still have CLIENT NODES section")
	s.Contains(diagram, "STORAGE NODES", "Diagram should still have STORAGE NODES section")
	s.Contains(diagram, "[storage]", "Diagram should still show storage service label")
	s.Contains(diagram, "[hf3fs_fuse]", "Diagram should still show hf3fs_fuse service label")
}

func (s *archDiagramSuite) TestGetClientNodes() {
	s.Equal(
		[]string{"192.168.1.3", "192.168.1.4"},
		s.generator.GetClientNodes(),
	)

	s.cfg.Services.Client = config.Client{}
	s.Equal([]string{}, s.generator.GetClientNodes())

}

func (s *archDiagramSuite) TestGetRenderableNodes() {
	s.Equal(
		[]string{"192.168.1.1", "192.168.1.2", "192.168.1.3"},
		s.generator.GetRenderableNodes(),
	)

}
