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
	"math"
	"strings"

	"github.com/open3fs/m3fs/pkg/config"
)

// ================ Interfaces ================

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

// ================ Data Provider ================

// ClusterDataProvider provides cluster node data for rendering architecture diagrams
type ClusterDataProvider struct {
	serviceNodeCountsFunc func() map[config.ServiceType]int
	clientNodesFunc       func() []string
	renderableNodesFunc   func() []string
	nodeServicesFunc      func(string) []string
	totalNodeCountFunc    func() int
	networkSpeedFunc      func() string
	networkTypeFunc       func() string
}

// NewClusterDataProvider creates a new ClusterDataProvider with the given functions
func NewClusterDataProvider(
	serviceNodeCountsFunc func() map[config.ServiceType]int,
	clientNodesFunc func() []string,
	renderableNodesFunc func() []string,
	nodeServicesFunc func(string) []string,
	totalNodeCountFunc func() int,
	networkSpeedFunc func() string,
	networkTypeFunc func() string,
) *ClusterDataProvider {
	return &ClusterDataProvider{
		serviceNodeCountsFunc: serviceNodeCountsFunc,
		clientNodesFunc:       clientNodesFunc,
		renderableNodesFunc:   renderableNodesFunc,
		nodeServicesFunc:      nodeServicesFunc,
		totalNodeCountFunc:    totalNodeCountFunc,
		networkSpeedFunc:      networkSpeedFunc,
		networkTypeFunc:       networkTypeFunc,
	}
}

// GetClientNodes implements NodeDataProvider
func (p *ClusterDataProvider) GetClientNodes() []string {
	return p.clientNodesFunc()
}

// GetRenderableNodes implements NodeDataProvider
func (p *ClusterDataProvider) GetRenderableNodes() []string {
	return p.renderableNodesFunc()
}

// GetNodeServices implements NodeDataProvider
func (p *ClusterDataProvider) GetNodeServices(nodeName string) []string {
	return p.nodeServicesFunc(nodeName)
}

// GetServiceNodeCounts implements NodeDataProvider
func (p *ClusterDataProvider) GetServiceNodeCounts() map[config.ServiceType]int {
	return p.serviceNodeCountsFunc()
}

// GetTotalNodeCount implements NodeDataProvider
func (p *ClusterDataProvider) GetTotalNodeCount() int {
	return p.totalNodeCountFunc()
}

// GetNetworkSpeed implements NodeDataProvider
func (p *ClusterDataProvider) GetNetworkSpeed() string {
	return p.networkSpeedFunc()
}

// GetNetworkType implements NodeDataProvider
func (p *ClusterDataProvider) GetNetworkType() string {
	return p.networkTypeFunc()
}

// ================ Adapter ================

// ArchDiagramAdapter is an adapter that connects a NodeDataProvider with an ArchDiagramRenderer
type ArchDiagramAdapter struct {
	dataProvider NodeDataProvider
	renderer     *ArchDiagramRenderer
}

// NewArchDiagramAdapter creates a new ArchDiagramAdapter
func NewArchDiagramAdapter(dataProvider NodeDataProvider, renderer *ArchDiagramRenderer) *ArchDiagramAdapter {
	return &ArchDiagramAdapter{
		dataProvider: dataProvider,
		renderer:     renderer,
	}
}

// Generate generates the architecture diagram
func (a *ArchDiagramAdapter) Generate() string {
	sb := &strings.Builder{}
	sb.Grow(1024)

	clientNodes := a.dataProvider.GetClientNodes()
	renderableNodes := a.dataProvider.GetRenderableNodes()

	nodeCount := int(math.Min(float64(len(clientNodes)), float64(a.renderer.Base.RowSize)))
	a.renderer.RenderHeader(sb, nodeCount)

	a.renderer.RenderClientSection(sb, clientNodes)

	a.renderer.RenderNetworkSection(sb, a.dataProvider.GetNetworkType(), a.dataProvider.GetNetworkSpeed())

	a.renderer.RenderStorageSection(sb, renderableNodes, a.dataProvider.GetNodeServices)

	a.renderer.RenderSummarySection(sb, a.getServiceNodeCountsFunc(), a.dataProvider.GetTotalNodeCount())

	return sb.String()
}

// getServiceNodeCountsFunc returns a function that provides service node counts
func (a *ArchDiagramAdapter) getServiceNodeCountsFunc() ServiceCountFunc {
	return func() map[config.ServiceType]int {
		return a.dataProvider.GetServiceNodeCounts()
	}
}

// SetColorEnabled enables or disables color output
func (a *ArchDiagramAdapter) SetColorEnabled(enabled bool) {
	if a.renderer != nil {
		a.renderer.SetColorEnabled(enabled)
	}
}

// Ensure ClusterDataProvider implements NodeDataProvider
var _ NodeDataProvider = (*ClusterDataProvider)(nil)
