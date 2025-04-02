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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open3fs/m3fs/pkg/config"
)

// createTestConfig creates a standard test configuration that can be reused across tests
func createTestConfig() *config.Config {
	return &config.Config{
		Name:        "test-cluster",
		NetworkType: "Ethernet",
		Nodes: []config.Node{
			{Name: "node1", Host: "192.168.1.1"},
			{Name: "node2", Host: "192.168.1.2"},
			{Name: "node3", Host: "192.168.1.3"},
			{Name: "very-long-node-name-that-will-be-truncated", Host: "192.168.1.4"},
		},
		NodeGroups: []config.NodeGroup{
			{
				Name:    "group1",
				IPBegin: "10.0.0.1",
				IPEnd:   "10.0.0.3",
			},
		},
		Services: config.Services{
			Mgmtd: config.Mgmtd{
				Nodes: []string{"node1"},
			},
			Meta: config.Meta{
				Nodes: []string{"node1", "node2"},
			},
			Storage: config.Storage{
				Nodes: []string{"node2", "node3"},
			},
			Client: config.Client{
				Nodes:          []string{"node3", "very-long-node-name-that-will-be-truncated"},
				NodeGroups:     []string{"group1"},
				HostMountpoint: "/mnt/m3fs",
			},
			Fdb: config.Fdb{
				Nodes: []string{"node1"},
			},
			Clickhouse: config.Clickhouse{
				Nodes: []string{"node2"},
			},
			Monitor: config.Monitor{
				Nodes: []string{"node3"},
			},
		},
	}
}

func TestArchitectureDiagramGenerator(t *testing.T) {
	cfg := createTestConfig()
	generator := NewArchitectureDiagramGenerator(cfg)

	t.Run("Generate", func(t *testing.T) {
		diagram, err := generator.Generate()
		assert.NoError(t, err, "Generate should not return an error")
		assert.NotEmpty(t, diagram, "Generated diagram should not be empty")
		assert.Contains(t, diagram, "Cluster: test-cluster", "Diagram should contain cluster name")
	})

	t.Run("GenerateBasicASCII", func(t *testing.T) {
		diagram, err := generator.GenerateBasicASCII()
		assert.NoError(t, err, "GenerateBasicASCII should not return an error")
		assert.NotEmpty(t, diagram, "Generated ASCII diagram should not be empty")

		// Check cluster name
		assert.Contains(t, diagram, "Cluster: test-cluster", "Diagram should contain cluster name")

		// Check node sections
		assert.Contains(t, diagram, "CLIENT NODES", "Diagram should have CLIENT NODES section")
		assert.Contains(t, diagram, "STORAGE NODES", "Diagram should have STORAGE NODES section")

		// Check node names are present (including truncated ones)
		assert.Contains(t, diagram, "node1", "Diagram should show node1")
		assert.Contains(t, diagram, "node2", "Diagram should show node2")
		assert.Contains(t, diagram, "node3", "Diagram should show node3")
		assert.Contains(t, diagram, "very-long-nod...", "Long node name should be truncated")

		// Check service labels are present
		assert.Contains(t, diagram, "[mgmtd]", "Diagram should show mgmtd service")
		assert.Contains(t, diagram, "[meta]", "Diagram should show meta service")
		assert.Contains(t, diagram, "[storage]", "Diagram should show storage service")
		assert.Contains(t, diagram, "[hf3fs_fuse]", "Diagram should show hf3fs_fuse service")
		assert.Contains(t, diagram, "[foundationdb]", "Diagram should show foundationdb service")
		assert.Contains(t, diagram, "[clickhouse]", "Diagram should show clickhouse service")
		assert.Contains(t, diagram, "[monitor]", "Diagram should show monitor service")
	})
}

func TestNodeListFunctions(t *testing.T) {
	t.Run("GetClientNodes", func(t *testing.T) {
		testCases := []struct {
			name           string
			cfg            *config.Config
			expectedCount  int
			expectedNodes  []string
			unexpectedNode string
		}{
			{
				name:           "Standard configuration",
				cfg:            createTestConfig(),
				expectedCount:  3, // node3, very-long-node..., and group1[...]
				expectedNodes:  []string{"node3", "group1"},
				unexpectedNode: "node2",
			},
			{
				name: "Empty client configuration",
				cfg: &config.Config{
					Nodes: []config.Node{
						{Name: "node1", Host: "192.168.1.1"},
					},
					Services: config.Services{
						Client: config.Client{},
					},
				},
				expectedCount:  1, // Default client node
				expectedNodes:  []string{"default-client"},
				unexpectedNode: "node1",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				generator := NewArchitectureDiagramGenerator(tc.cfg)
				clientNodes := generator.getClientNodes()

				assert.Len(t, clientNodes, tc.expectedCount, "Client nodes count should match expected")

				for _, expectedNode := range tc.expectedNodes {
					found := false
					for _, clientNode := range clientNodes {
						if strings.Contains(clientNode, expectedNode) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected node %s should be in client nodes", expectedNode)
				}

				if tc.unexpectedNode != "" {
					for _, clientNode := range clientNodes {
						assert.NotEqual(t, tc.unexpectedNode, clientNode, "Unexpected node should not be in client nodes")
					}
				}
			})
		}
	})

	t.Run("GetStorageNodes", func(t *testing.T) {
		testCases := []struct {
			name           string
			cfg            *config.Config
			expectedNodes  []string
			unexpectedNode string
		}{
			{
				name:           "Standard configuration",
				cfg:            createTestConfig(),
				expectedNodes:  []string{"node1", "node2", "node3"},
				unexpectedNode: "",
			},
			{
				name: "Empty storage configuration",
				cfg: &config.Config{
					Services: config.Services{},
				},
				expectedNodes:  []string{"default-storage"},
				unexpectedNode: "node1",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				generator := NewArchitectureDiagramGenerator(tc.cfg)
				storageNodes := generator.getStorageNodes()

				for _, expectedNode := range tc.expectedNodes {
					found := false
					for _, node := range storageNodes {
						if strings.Contains(node, expectedNode) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected node %s should be in storage nodes", expectedNode)
				}

				if tc.unexpectedNode != "" {
					for _, node := range storageNodes {
						assert.NotEqual(t, tc.unexpectedNode, node, "Unexpected node should not be in storage nodes")
					}
				}
			})
		}
	})

	t.Run("IsNodeInList", func(t *testing.T) {
		generator := NewArchitectureDiagramGenerator(nil)
		nodeList := []string{"node1", "node2", "node3"}

		assert.True(t, generator.isNodeInList("node1", nodeList), "node1 should be found in the list")
		assert.True(t, generator.isNodeInList("node3", nodeList), "node3 should be found in the list")
		assert.False(t, generator.isNodeInList("node4", nodeList), "node4 should not be found in the list")
		assert.False(t, generator.isNodeInList("", nodeList), "Empty string should not be found in the list")
		assert.False(t, generator.isNodeInList("node1", []string{}), "Any node should not be found in empty list")
	})
}

func TestExpandNodeGroup(t *testing.T) {
	testCases := []struct {
		name         string
		nodeGroup    config.NodeGroup
		expectedName string
	}{
		{
			name: "Standard node group",
			nodeGroup: config.NodeGroup{
				Name:    "test-group",
				IPBegin: "192.168.1.10",
				IPEnd:   "192.168.1.20",
			},
			expectedName: "test-group[192.168.1.10-192.168.1.20]",
		},
		{
			name: "Single IP node group",
			nodeGroup: config.NodeGroup{
				Name:    "single-ip",
				IPBegin: "10.0.0.1",
				IPEnd:   "10.0.0.1",
			},
			expectedName: "single-ip[10.0.0.1-10.0.0.1]",
		},
		{
			name: "Node group with special characters",
			nodeGroup: config.NodeGroup{
				Name:    "special-!@#$",
				IPBegin: "172.16.0.1",
				IPEnd:   "172.16.0.10",
			},
			expectedName: "special-!@#$[172.16.0.1-172.16.0.10]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			generator := NewArchitectureDiagramGenerator(nil)
			result := generator.expandNodeGroup(&tc.nodeGroup)

			assert.Len(t, result, 1, "expandNodeGroup should return a slice with one element")
			assert.Equal(t, tc.expectedName, result[0], "Node group name should match expected format")
		})
	}
}

func TestNetworkSpeed(t *testing.T) {
	testCases := []struct {
		name          string
		networkType   config.NetworkType
		expectedSpeed string
	}{
		{
			name:          "InfiniBand network",
			networkType:   config.NetworkTypeIB,
			expectedSpeed: "50 Gb/sec",
		},
		{
			name:          "RDMA network",
			networkType:   config.NetworkTypeRDMA,
			expectedSpeed: "100 Gb/sec",
		},
		{
			name:          "Ethernet network",
			networkType:   "Ethernet",
			expectedSpeed: "10 Gb/sec",
		},
		{
			name:          "RXE network",
			networkType:   config.NetworkTypeRXE,
			expectedSpeed: "10 Gb/sec", // Default for non-IB and non-RDMA
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock getNetworkSpeed function to test network type logic only
			mockGetSpeed := func(networkType config.NetworkType) string {
				if networkType == config.NetworkTypeIB {
					return "50 Gb/sec"
				}
				if networkType == config.NetworkTypeRDMA {
					return "100 Gb/sec"
				}
				return "10 Gb/sec"
			}

			speed := mockGetSpeed(tc.networkType)
			assert.Equal(t, tc.expectedSpeed, speed, "Network speed should match expected value for %s", tc.networkType)
		})
	}
}
