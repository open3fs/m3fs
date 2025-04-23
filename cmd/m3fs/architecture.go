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
	"sort"
	"sync"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/network"
	"github.com/open3fs/m3fs/pkg/render"
	"github.com/open3fs/m3fs/pkg/utils"
	"github.com/sirupsen/logrus"
)

// ===== Constants and Type Definitions =====

// NodeResult represents the result of node processing
type NodeResult struct {
	Name  config.ServiceType
	Count int
}

// ArchNodeResults is a slice of NodeResult for service node counts
type ArchNodeResults []NodeResult

// Service display names mapping
var serviceDisplayNames = map[config.ServiceType]string{
	config.ServiceStorage:    "storage",
	config.ServiceFdb:        "foundationdb",
	config.ServiceMeta:       "meta",
	config.ServiceMgmtd:      "mgmtd",
	config.ServiceMonitor:    "monitor",
	config.ServiceClickhouse: "clickhouse",
	config.ServiceClient:     "client",
}

// Service types for iteration
var serviceTypes = []config.ServiceType{
	config.ServiceStorage,
	config.ServiceFdb,
	config.ServiceMeta,
	config.ServiceMgmtd,
	config.ServiceMonitor,
	config.ServiceClickhouse,
	config.ServiceClient,
}

// Service config accessor functions map
var serviceConfigMap = map[config.ServiceType]func(cfg *config.Config) ([]string, []string){
	config.ServiceStorage: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Storage.Nodes, cfg.Services.Storage.NodeGroups
	},
	config.ServiceFdb: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Fdb.Nodes, cfg.Services.Fdb.NodeGroups
	},
	config.ServiceMeta: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Meta.Nodes, cfg.Services.Meta.NodeGroups
	},
	config.ServiceMgmtd: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Mgmtd.Nodes, cfg.Services.Mgmtd.NodeGroups
	},
	config.ServiceMonitor: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Monitor.Nodes, cfg.Services.Monitor.NodeGroups
	},
	config.ServiceClickhouse: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Clickhouse.Nodes, cfg.Services.Clickhouse.NodeGroups
	},
	config.ServiceClient: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Client.Nodes, cfg.Services.Client.NodeGroups
	},
}

// NewConfigError creates a configuration error with the given message
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

// ===== Constructors and Core Functions =====

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

// setDefaultConfig sets cluster.yml values for configuration
func setDefaultConfig(cfg *config.Config) *config.Config {
	if cfg.Name == "" {
		cfg.Name = "cluster.yml"
	}
	if cfg.NetworkType == "" {
		cfg.NetworkType = "ethernet"
	}
	return cfg
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

// ===== Network Related Methods =====

// GetNetworkType returns the type of network being used
func (g *ArchDiagram) GetNetworkType() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.cfg == nil {
		return "ethernet"
	}
	return string(g.cfg.NetworkType)
}

// GetNetworkSpeed returns the network speed for the diagram
func (g *ArchDiagram) GetNetworkSpeed() string {
	return g.getNetworkSpeed()
}

// getNetworkSpeed returns the actual network speed based on network type
func (g *ArchDiagram) getNetworkSpeed() string {
	g.mu.RLock()
	networkType := g.cfg.NetworkType
	g.mu.RUnlock()

	return network.GetNetworkSpeed(string(networkType))
}

// ===== Node Basic Operations =====

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

// ===== Node Count Methods =====

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
		if node.Host != "" && net.ParseIP(node.Host) != nil {
			uniqueIPs[node.Host] = struct{}{}
		}
	}

	// Add nodes from node groups
	for _, nodeGroup := range g.cfg.NodeGroups {
		ipList := g.expandNodeGroup(&nodeGroup)
		for _, ip := range ipList {
			uniqueIPs[ip] = struct{}{}
		}
	}

	return len(uniqueIPs)
}

// GetServiceNodeCounts returns counts of nodes by service type
func (g *ArchDiagram) GetServiceNodeCounts() map[config.ServiceType]int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.cfg == nil {
		return nil
	}

	result := make(map[config.ServiceType]int)
	for _, service := range serviceTypes {
		nodes := g.getServiceNodesInternal(service)
		if len(nodes) > 0 {
			result[service] = len(nodes)
		}
	}
	return result
}

// GetServiceNodeCountsDetail returns detailed counts of nodes by service type
func (g *ArchDiagram) GetServiceNodeCountsDetail() ArchNodeResults {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.cfg == nil {
		return nil
	}

	result := make(ArchNodeResults, 0, len(serviceTypes))
	for _, service := range serviceTypes {
		nodes := g.getServiceNodesInternal(service)
		if len(nodes) > 0 {
			result = append(result, NodeResult{
				Name:  service,
				Count: len(nodes),
			})
		}
	}
	return result
}

// ===== Node Retrieval Methods =====

// sortNodesByIP sorts a slice of IP address strings
func (g *ArchDiagram) sortNodesByIP(nodes []string) []string {
	if len(nodes) <= 1 {
		return nodes
	}

	sorted := make([]string, len(nodes))
	copy(sorted, nodes)

	sort.Slice(sorted, func(i, j int) bool {
		return utils.CompareIPAddresses(sorted[i], sorted[j]) < 0
	})

	return sorted
}

// GetClientNodes returns client nodes
func (g *ArchDiagram) GetClientNodes() []string {
	nodes := g.getServiceNodes(config.ServiceClient)
	return g.sortNodesByIP(nodes)
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

// getNodeServices returns the services running on a node
func (g *ArchDiagram) getNodeServices(node string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.cfg == nil {
		return nil
	}

	services := make([]string, 0, len(serviceTypes))

	for _, svcType := range serviceTypes {
		// Skip client service in storage node display
		if svcType == config.ServiceClient {
			continue
		}

		if g.checkNodeService(node, svcType) {
			displayName := serviceDisplayNames[svcType]
			services = append(services, fmt.Sprintf("[%s]", displayName))
		}
	}
	return services
}

// ===== Node List Building Methods =====

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
	// First try nodes defined in the group
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

	// Try IP range
	ipList, err := utils.GenerateIPRange(nodeGroup.IPBegin, nodeGroup.IPEnd)
	if err != nil {
		logrus.Errorf("Failed to expand node group %s: %v", nodeGroup.Name, err)
		return []string{}
	}

	return ipList
}

// ===== Service Configuration Methods =====

// getServiceConfig returns nodes and node groups for a service type
func (g *ArchDiagram) getServiceConfig(serviceType config.ServiceType) ([]string, []string) {
	if g.cfg == nil {
		return nil, nil
	}

	configFunc, ok := serviceConfigMap[serviceType]
	if !ok {
		logrus.Errorf("Unknown service type: %s", serviceType)
		return nil, nil
	}

	return configFunc(g.cfg)
}

// getServiceNodeList returns nodes for a service without locking
func (g *ArchDiagram) getServiceNodeList(nodes []string, nodeGroups []string) ([]string, error) {
	if g.cfg == nil {
		return nil, errors.New("configuration is nil")
	}

	serviceNodesMap := make(map[string]struct{})

	// Create node name to host mapping
	nodeLookup := make(map[string]string)
	for _, node := range g.cfg.Nodes {
		if node.Host != "" && net.ParseIP(node.Host) != nil {
			nodeLookup[node.Name] = node.Host
		}
	}

	// Add direct nodes
	for _, nodeName := range nodes {
		if nodeIP, ok := nodeLookup[nodeName]; ok {
			serviceNodesMap[nodeIP] = struct{}{}
		}
	}

	// Create node group map
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
				serviceNodesMap[ip] = struct{}{}
			}
		} else {
			logrus.Debugf("Node group %s not found in configuration", groupName)
		}
	}

	// Build result
	serviceNodes := make([]string, 0, len(serviceNodesMap))
	for ip := range serviceNodesMap {
		serviceNodes = append(serviceNodes, ip)
	}

	return serviceNodes, nil
}

// ===== Rendering Related Methods =====

// GetRenderableNodes returns service nodes to render in the diagram
func (g *ArchDiagram) GetRenderableNodes() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	allNodes := g.buildOrderedNodeList()
	if len(allNodes) == 0 {
		return nil
	}

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

	return g.sortNodesByIP(renderableNodes)
}
