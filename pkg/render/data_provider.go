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
	"github.com/open3fs/m3fs/pkg/config"
)

// NodeDataProvider provides node data for architecture diagrams
type NodeDataProvider interface {
	// GetClientNodes returns all client nodes
	GetClientNodes() []string

	// GetRenderableNodes returns nodes to render in the diagram
	GetRenderableNodes() []string

	// GetNodeServices returns services running on a node
	GetNodeServices(nodeName string) []string

	// GetServiceNodeCounts returns node counts by service type
	GetServiceNodeCounts() map[config.ServiceType]int

	// GetTotalNodeCount returns the total number of nodes
	GetTotalNodeCount() int

	// GetNetworkSpeed returns the network speed
	GetNetworkSpeed() string

	// GetNetworkType returns the network type
	GetNetworkType() string
}

// ClusterDataProvider provides cluster node data for rendering
type ClusterDataProvider struct {
	getServiceNodeCountsFunc func() map[config.ServiceType]int
	getClientNodesFunc       func() []string
	getRenderableNodesFunc   func() []string
	getNodeServicesFunc      func(string) []string
	getTotalNodeCountFunc    func() int
	getNetworkSpeedFunc      func() string
	getNetworkTypeFunc       func() string
}

// NewClusterDataProvider creates a new ClusterDataProvider
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
		getServiceNodeCountsFunc: serviceNodeCountsFunc,
		getClientNodesFunc:       clientNodesFunc,
		getRenderableNodesFunc:   renderableNodesFunc,
		getNodeServicesFunc:      nodeServicesFunc,
		getTotalNodeCountFunc:    totalNodeCountFunc,
		getNetworkSpeedFunc:      networkSpeedFunc,
		getNetworkTypeFunc:       networkTypeFunc,
	}
}

// GetClientNodes returns all client nodes
func (p *ClusterDataProvider) GetClientNodes() []string {
	return p.getClientNodesFunc()
}

// GetRenderableNodes returns nodes to render in the diagram
func (p *ClusterDataProvider) GetRenderableNodes() []string {
	return p.getRenderableNodesFunc()
}

// GetNodeServices returns services running on a node
func (p *ClusterDataProvider) GetNodeServices(nodeName string) []string {
	return p.getNodeServicesFunc(nodeName)
}

// GetServiceNodeCounts returns node counts by service type
func (p *ClusterDataProvider) GetServiceNodeCounts() map[config.ServiceType]int {
	return p.getServiceNodeCountsFunc()
}

// GetTotalNodeCount returns the total number of nodes
func (p *ClusterDataProvider) GetTotalNodeCount() int {
	return p.getTotalNodeCountFunc()
}

// GetNetworkSpeed returns the network speed
func (p *ClusterDataProvider) GetNetworkSpeed() string {
	return p.getNetworkSpeedFunc()
}

// GetNetworkType returns the network type
func (p *ClusterDataProvider) GetNetworkType() string {
	return p.getNetworkTypeFunc()
}
