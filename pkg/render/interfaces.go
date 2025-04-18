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

package render

import (
	"strings"

	"github.com/open3fs/m3fs/pkg/config"
)

// ArchDiagramGenerator defines the interface for generating architecture diagrams
type ArchDiagramGenerator interface {
	// Generate generates the architecture diagram
	Generate() string
}

// NodeDataProvider defines the interface for providing node data to a renderer
type NodeDataProvider interface {
	// GetClientNodes returns client nodes
	GetClientNodes() []string

	// GetRenderableNodes returns all nodes that should be rendered in the diagram
	GetRenderableNodes() []string

	// GetNodeServices returns the services running on a node
	GetNodeServices(nodeName string) []string

	// GetServiceNodeCounts returns the count of nodes for each service type
	GetServiceNodeCounts() map[config.ServiceType]int

	// GetTotalNodeCount returns the total number of nodes
	GetTotalNodeCount() int

	// GetNetworkSpeed returns the network speed
	GetNetworkSpeed() string

	// GetNetworkType returns the network type
	GetNetworkType() string
}

// ArchDiagramRendererInterface is an interface for rendering architecture diagrams
type ArchDiagramRendererInterface interface {
	// RenderHeader renders the diagram header
	RenderHeader(sb *strings.Builder)

	// RenderClientSection renders the client nodes section
	RenderClientSection(sb *strings.Builder, clientNodes []string)

	// RenderNetworkSection renders the network section
	RenderNetworkSection(sb *strings.Builder, networkType string, networkSpeed string)

	// RenderStorageSection renders the storage nodes section
	RenderStorageSection(sb *strings.Builder, storageNodes []string, nodeServicesFunc NodeServicesFunc)

	// RenderSummarySection renders the summary section
	RenderSummarySection(sb *strings.Builder, serviceCountsFunc ServiceCountFunc, totalNodeCount int)
}
