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
	"fmt"
	"net"
	"sync"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/network"
	"github.com/open3fs/m3fs/pkg/render"
	"github.com/open3fs/m3fs/pkg/utils"
	"github.com/sirupsen/logrus"
)

// ================ Type Definitions and Constants ================

// NodeResult represents the result of node processing
type NodeResult struct {
	Index     int
	NodeName  string
	IsStorage bool
}

// Service display names mapping
var serviceDisplayNames = map[config.ServiceType]string{
	config.ServiceStorage:    "storage",
	config.ServiceFdb:        "foundationdb",
	config.ServiceMeta:       "meta",
	config.ServiceMgmtd:      "mgmtd",
	config.ServiceMonitor:    "monitor",
	config.ServiceClickhouse: "clickhouse",
}

// Service types for iteration
var serviceTypes = []config.ServiceType{
	config.ServiceStorage,
	config.ServiceFdb,
	config.ServiceMeta,
	config.ServiceMgmtd,
	config.ServiceMonitor,
	config.ServiceClickhouse,
}

// NewConfigError creates a configuration error
func NewConfigError(msg string) error {
	return errors.New(msg)
}

// NewNetworkError creates a network error with operation context
func NewNetworkError(operation string, err error) error {
	return errors.Annotatef(err, "%s failed", operation)
}

// NewServiceError creates a service error with service type context
func NewServiceError(serviceType config.ServiceType, err error) error {
	return errors.Annotatef(err, "service %s error", serviceType)
}

// ArchDiagram generates architecture diagrams for m3fs clusters
type ArchDiagram struct {
	cfg          *config.Config
	renderer     *render.DiagramRenderer
	archRenderer *render.ArchRenderer
	dataProvider *render.ClusterDataProvider

	mu sync.RWMutex
}

//
// Core functionality
//

// NewArchDiagram creates a new ArchDiagram with default configuration
func NewArchDiagram(cfg *config.Config) *ArchDiagram {
	if cfg == nil {
		logrus.Warn("Creating ArchDiagram with nil config")
		cfg = &config.Config{
			Name:        "default",
			NetworkType: "ethernet",
		}
	}

	cfg = setDefaultConfig(cfg)
	baseRenderer := render.NewDiagramRenderer(cfg)

	archDiagram := &ArchDiagram{
		cfg:      cfg,
		renderer: baseRenderer,
	}

	dataProvider := render.NewClusterDataProvider(
		archDiagram.GetServiceNodeCounts,
		archDiagram.GetClientNodes,
		archDiagram.GetRenderableNodes,
		archDiagram.getNodeServices,
		archDiagram.GetTotalNodeCount,
		archDiagram.getNetworkSpeed,
		archDiagram.GetNetworkType,
	)

	archDiagram.dataProvider = dataProvider
	archDiagram.archRenderer = render.NewArchRenderer(baseRenderer, dataProvider)

	return archDiagram
}

// Generate generates an architecture diagram
func (g *ArchDiagram) Generate() string {
	if g.cfg == nil {
		return "Error: No configuration provided"
	}

	return g.archRenderer.Generate()
}

// SetColorEnabled enables or disables color output in the diagram
func (g *ArchDiagram) SetColorEnabled(enabled bool) {
	g.archRenderer.SetColorEnabled(enabled)
}

// setDefaultConfig sets default values for configuration
func setDefaultConfig(cfg *config.Config) *config.Config {
	if cfg.Name == "" {
		cfg.Name = "default"
	}
	if cfg.NetworkType == "" {
		cfg.NetworkType = "ethernet"
	}
	return cfg
}

//
// Network related methods
//

// GetNetworkType implements render.NodeDataProvider
func (g *ArchDiagram) GetNetworkType() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.cfg == nil {
		return "ethernet"
	}
	return string(g.cfg.NetworkType)
}

// GetNetworkSpeed implements render.NodeDataProvider
func (g *ArchDiagram) GetNetworkSpeed() string {
	return g.getNetworkSpeed()
}

// getNetworkSpeed returns the network speed
func (g *ArchDiagram) getNetworkSpeed() string {
	g.mu.RLock()
	networkType := g.cfg.NetworkType
	g.mu.RUnlock()

	return network.GetNetworkSpeed(string(networkType))
}

//
// Node basic operations
//

// isNodeInList checks if a node is in a list
func (g *ArchDiagram) isNodeInList(nodeName string, nodeList []string) bool {
	for _, node := range nodeList {
		if node == nodeName {
			return true
		}
	}
	return false
}

// checkNodeService checks if a node belongs to a service (without locking)
func (g *ArchDiagram) checkNodeService(nodeName string, serviceType config.ServiceType) bool {
	nodes := g.getServiceNodesInternal(serviceType)
	return g.isNodeInList(nodeName, nodes)
}

//
// Node retrieval methods
//

// GetClientNodes returns client nodes
func (g *ArchDiagram) GetClientNodes() []string {
	return g.getServiceNodes(config.ServiceClient)
}

// getServiceNodes returns nodes for a specific service type
func (g *ArchDiagram) getServiceNodes(serviceType config.ServiceType) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.getServiceNodesInternal(serviceType)
}

// getServiceNodesInternal returns service nodes without locking
func (g *ArchDiagram) getServiceNodesInternal(serviceType config.ServiceType) []string {
	if g.cfg == nil {
		return nil
	}

	nodes, nodeGroups := g.getServiceConfig(serviceType)
	result, err := g.getServiceNodeList(nodes, nodeGroups)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to get nodes for service %s", serviceType)
		return nil
	}
	return result
}

// GetTotalNodeCount returns the total number of actual nodes
func (g *ArchDiagram) GetTotalNodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.cfg == nil {
		return 0
	}

	uniqueIPs := make(map[string]struct{})

	// Add direct nodes
	for _, node := range g.cfg.Nodes {
		uniqueIPs[node.Host] = struct{}{}
	}

	// Add nodes from node groups
	for _, nodeGroup := range g.cfg.NodeGroups {
		ipList, err := utils.GenerateIPRange(nodeGroup.IPBegin, nodeGroup.IPEnd)
		if err != nil {
			continue
		}

		for _, ip := range ipList {
			uniqueIPs[ip] = struct{}{}
		}
	}

	return len(uniqueIPs)
}

//
// Node list building methods
//

// buildOrderedNodeList builds a list of nodes ordered by config appearance
func (g *ArchDiagram) buildOrderedNodeList() []string {
	if g.cfg == nil {
		return nil
	}

	nodeMap := make(map[string]struct{})
	allNodes := make([]string, 0, len(g.cfg.Nodes))

	// First add direct nodes
	for _, node := range g.cfg.Nodes {
		if node.Host != "" && net.ParseIP(node.Host) != nil {
			if _, exists := nodeMap[node.Host]; !exists {
				nodeMap[node.Host] = struct{}{}
				allNodes = append(allNodes, node.Host)
			}
		}
	}

	// Then add node groups
	for _, nodeGroup := range g.cfg.NodeGroups {
		ipList := g.expandNodeGroup(&nodeGroup)
		for _, ip := range ipList {
			if _, exists := nodeMap[ip]; !exists {
				nodeMap[ip] = struct{}{}
				allNodes = append(allNodes, ip)
			}
		}
	}

	return allNodes
}

// expandNodeGroup expands a node group into individual nodes
func (g *ArchDiagram) expandNodeGroup(nodeGroup *config.NodeGroup) []string {
	if len(nodeGroup.Nodes) > 0 {
		ipList := make([]string, 0, len(nodeGroup.Nodes))
		for _, node := range nodeGroup.Nodes {
			if node.Host != "" && net.ParseIP(node.Host) != nil {
				ipList = append(ipList, node.Host)
			}
		}
		if len(ipList) > 0 {
			return ipList
		}
	}

	ipList, err := utils.GenerateIPRange(nodeGroup.IPBegin, nodeGroup.IPEnd)
	if err != nil {
		logrus.Errorf("Failed to expand node group %s: %v", nodeGroup.Name, err)
		return []string{}
	}

	return ipList
}

//
// Service configuration methods
//

// getServiceConfig returns nodes and node groups for a service type
func (g *ArchDiagram) getServiceConfig(serviceType config.ServiceType) ([]string, []string) {
	if g.cfg == nil {
		return nil, nil
	}

	switch serviceType {
	case config.ServiceMgmtd:
		return g.cfg.Services.Mgmtd.Nodes, g.cfg.Services.Mgmtd.NodeGroups
	case config.ServiceMonitor:
		return g.cfg.Services.Monitor.Nodes, g.cfg.Services.Monitor.NodeGroups
	case config.ServiceStorage:
		return g.cfg.Services.Storage.Nodes, g.cfg.Services.Storage.NodeGroups
	case config.ServiceFdb:
		return g.cfg.Services.Fdb.Nodes, g.cfg.Services.Fdb.NodeGroups
	case config.ServiceClickhouse:
		return g.cfg.Services.Clickhouse.Nodes, g.cfg.Services.Clickhouse.NodeGroups
	case config.ServiceMeta:
		return g.cfg.Services.Meta.Nodes, g.cfg.Services.Meta.NodeGroups
	case config.ServiceClient:
		return g.cfg.Services.Client.Nodes, g.cfg.Services.Client.NodeGroups
	default:
		logrus.Errorf("Unknown service type: %s", serviceType)
		return nil, nil
	}
}

// getServiceNodeList returns nodes for a service without locking
func (g *ArchDiagram) getServiceNodeList(nodes []string, nodeGroups []string) ([]string, error) {
	if g.cfg == nil {
		return nil, errors.New("configuration is nil")
	}

	serviceNodes := make([]string, 0)
	nodeMap := make(map[string]struct{})

	// Add direct nodes
	for _, nodeName := range nodes {
		for _, node := range g.cfg.Nodes {
			if node.Name == nodeName {
				if node.Host != "" && net.ParseIP(node.Host) != nil {
					if _, exists := nodeMap[node.Host]; !exists {
						nodeMap[node.Host] = struct{}{}
						serviceNodes = append(serviceNodes, node.Host)
					}
				}
				break
			}
		}
	}

	// Create node group map for quick lookup
	nodeGroupMap := make(map[string]*config.NodeGroup)
	for i := range g.cfg.NodeGroups {
		nodeGroup := &g.cfg.NodeGroups[i]
		nodeGroupMap[nodeGroup.Name] = nodeGroup
	}

	// Add nodes from node groups
	for _, groupName := range nodeGroups {
		if nodeGroup, found := nodeGroupMap[groupName]; found {
			ipList := g.expandNodeGroup(nodeGroup)
			for _, ip := range ipList {
				if _, exists := nodeMap[ip]; !exists {
					nodeMap[ip] = struct{}{}
					serviceNodes = append(serviceNodes, ip)
				}
			}
		} else {
			logrus.Debugf("Node group %s not found in configuration", groupName)
		}
	}

	return serviceNodes, nil
}

// GetServiceNodeCounts returns counts of nodes by service type
func (g *ArchDiagram) GetServiceNodeCounts() map[config.ServiceType]int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	counts := make(map[config.ServiceType]int)

	for _, svcType := range serviceTypes {
		nodes := g.getServiceNodesInternal(svcType)
		counts[svcType] = len(nodes)
	}

	clientNodes := g.getServiceNodesInternal(config.ServiceClient)
	counts[config.ServiceClient] = len(clientNodes)

	return counts
}

// getNodeServices returns the services running on a node
func (g *ArchDiagram) getNodeServices(node string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	services := make([]string, 0)

	for _, svcType := range serviceTypes {
		if g.checkNodeService(node, svcType) {
			displayName := serviceDisplayNames[svcType]
			services = append(services, fmt.Sprintf("[%s]", displayName))
		}
	}
	return services
}

//
// Rendering related methods
//

// GetRenderableNodes returns service nodes to render in the architecture diagram
// excluding client-only nodes
func (g *ArchDiagram) GetRenderableNodes() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	allNodes := g.buildOrderedNodeList()
	if len(allNodes) == 0 {
		return nil
	}

	// Use simple sequential processing for small node counts
	renderableNodes := make([]string, 0, len(allNodes))
	for _, node := range allNodes {
		hasService := false
		for _, svcType := range serviceTypes {
			if svcType != config.ServiceClient && g.checkNodeService(node, svcType) {
				hasService = true
				break
			}
		}

		if hasService {
			renderableNodes = append(renderableNodes, node)
		}
	}

	return renderableNodes
}
