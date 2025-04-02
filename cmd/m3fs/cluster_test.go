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

	"github.com/stretchr/testify/assert"

	"github.com/open3fs/m3fs/pkg/config"
)

// TestDrawClusterArchitecture tests the architecture diagram generation functionality
func TestDrawClusterArchitecture(t *testing.T) {
	testCases := []struct {
		name           string
		configSetup    func() *config.Config
		expectedItems  []string
		unexpectedItem string
	}{
		{
			name: "Standard configuration",
			configSetup: func() *config.Config {
				cfg := config.NewConfigWithDefaults()
				cfg.Name = "test-arch-cluster"
				cfg.NetworkType = "Ethernet"
				cfg.Nodes = []config.Node{
					{Name: "node1", Host: "192.168.1.1"},
					{Name: "node2", Host: "192.168.1.2"},
				}
				cfg.Services.Mgmtd.Nodes = []string{"node1"}
				cfg.Services.Storage.Nodes = []string{"node1", "node2"}
				cfg.Services.Client.Nodes = []string{"node2"}
				cfg.Services.Client.HostMountpoint = "/mnt/m3fs"
				return cfg
			},
			expectedItems: []string{
				"Cluster: test-arch-cluster",
				"CLIENT NODES",
				"STORAGE NODES",
				"node1",
				"node2",
				"[storage]",
				"[hf3fs_fuse]",
				"[mgmtd]",
			},
			unexpectedItem: "node3",
		},
		{
			name: "Empty configuration",
			configSetup: func() *config.Config {
				cfg := config.NewConfigWithDefaults()
				cfg.Name = "empty-cluster"
				return cfg
			},
			expectedItems: []string{
				"Cluster: empty-cluster",
				"CLIENT NODES",
				"STORAGE NODES",
				"default-client",
				"default-storage",
			},
			unexpectedItem: "node1",
		},
		{
			name: "Special characters in cluster name",
			configSetup: func() *config.Config {
				cfg := config.NewConfigWithDefaults()
				cfg.Name = "test@cluster!123"
				return cfg
			},
			expectedItems: []string{
				"Cluster: test@cluster!123",
			},
			unexpectedItem: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.configSetup()
			diagram := generateTestArchitectureDiagram(t, cfg)

			// Verify expected items are present
			for _, item := range tc.expectedItems {
				assert.Contains(t, diagram, item, "Diagram should contain %s", item)
			}

			// Verify unexpected items are not present
			if tc.unexpectedItem != "" {
				assert.NotContains(t, diagram, tc.unexpectedItem, "Diagram should not contain %s", tc.unexpectedItem)
			}
		})
	}
}

// generateTestArchitectureDiagram is a helper function to generate architecture diagram for tests
func generateTestArchitectureDiagram(t *testing.T, cfg *config.Config) string {
	diagramGenerator := NewArchitectureDiagramGenerator(cfg)
	diagram, err := diagramGenerator.GenerateBasicASCII()
	assert.NoError(t, err, "GenerateBasicASCII should not return an error")
	return diagram
}
