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

// Ensure ClusterDataProvider implements NodeDataProvider
var _ NodeDataProvider = (*ClusterDataProvider)(nil)
