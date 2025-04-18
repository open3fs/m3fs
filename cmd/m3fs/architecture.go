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
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/open3fs/m3fs/pkg/cache"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/network"
	"github.com/open3fs/m3fs/pkg/render"
	"github.com/open3fs/m3fs/pkg/utils"
	"github.com/sirupsen/logrus"
)

// ================ Type Definitions and Constants ================

// Concurrency constants
const (
	maxWorkers = 10
	timeout    = 30 * time.Second

	// Initial capacities
	initialStringBuilderCapacity = 1024
	initialMapCapacity           = 16
	initialSliceCapacity         = 16
)

// serviceDisplayNames is a map of service types to their display names
var serviceDisplayNames = map[config.ServiceType]string{
	config.ServiceStorage:    "storage",
	config.ServiceFdb:        "foundationdb",
	config.ServiceMeta:       "meta",
	config.ServiceMgmtd:      "mgmtd",
	config.ServiceMonitor:    "monitor",
	config.ServiceClickhouse: "clickhouse",
}

// serviceTypes is a pre-allocated slice of service types to avoid reallocations
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
	archRenderer *render.ArchDiagramRenderer
	cacheManager *cache.Manager
	nodeCache    *cache.NodeCache
	dataProvider *render.ClusterDataProvider

	mu sync.RWMutex
}

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
	cacheManager := cache.NewCacheManager(cache.DefaultCacheConfig)
	nodeCache := cache.NewNodeCache(cacheManager)

	archDiagram := &ArchDiagram{
		cfg:          cfg,
		renderer:     baseRenderer,
		archRenderer: render.NewArchDiagramRenderer(baseRenderer),
		cacheManager: cacheManager,
		nodeCache:    nodeCache,
	}

	archDiagram.dataProvider = render.NewClusterDataProvider(
		archDiagram.GetServiceNodeCounts,
		archDiagram.GetClientNodes,
		archDiagram.GetRenderableNodes,
		archDiagram.getNodeServices,
		archDiagram.GetTotalNodeCount,
		archDiagram.getNetworkSpeed,
		archDiagram.GetNetworkType,
	)

	return archDiagram
}

// Generate generates an architecture diagram
func (g *ArchDiagram) Generate() string {
	if g.cfg == nil {
		return "Error: No configuration provided"
	}

	adapter := render.NewArchDiagramAdapter(g.dataProvider, g.archRenderer)
	return adapter.Generate()
}

// SetColorEnabled enables or disables color output in the diagram
func (g *ArchDiagram) SetColorEnabled(enabled bool) {
	g.renderer.SetColorEnabled(enabled)
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

// ================ Interface Implementation Methods ================

// GetClientNodes returns client nodes
func (g *ArchDiagram) GetClientNodes() []string {
	return g.getServiceNodes(config.ServiceClient)
}

// GetServiceNodeCounts returns counts of nodes by service type
func (g *ArchDiagram) GetServiceNodeCounts() map[config.ServiceType]int {
	clientNodes := g.GetClientNodes()
	serviceNodesMap := g.prepareServiceNodesMap(clientNodes)
	return g.countServiceNodes(serviceNodesMap)
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

// GetNetworkType implements render.NodeDataProvider
func (g *ArchDiagram) GetNetworkType() string {
	if g.cfg == nil {
		return "ethernet"
	}
	return string(g.cfg.NetworkType)
}

// ================ Node Processing Methods ================

// GetRenderableNodes returns all service nodes that need to be rendered in the architecture diagram
// Including nodes with storage, meta, mgmtd, etc. services, but excluding client-only nodes
func (g *ArchDiagram) GetRenderableNodes() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	allNodes := g.buildOrderedNodeList()
	if len(allNodes) == 0 {
		return nil
	}

	// Process nodes in parallel for better performance
	results := g.processNodesInParallel(allNodes)
	if len(results) == 0 {
		return nil
	}

	// Extract nodes with any service (excluding client-only nodes)
	allNodes = g.extractRenderableNodes(results)

	clientNodes := g.GetClientNodes()
	clientNodeMap := make(map[string]struct{}, len(clientNodes))
	for _, node := range clientNodes {
		clientNodeMap[node] = struct{}{}
	}

	// Filter out nodes that are only client nodes and have no other services
	renderableNodes := make([]string, 0, len(allNodes))
	for _, node := range allNodes {
		hasService := false
		for _, svcType := range serviceTypes {
			if svcType != config.ServiceClient && g.checkNodeInService(node, svcType) {
				hasService = true
				break
			}
		}

		if hasService {
			renderableNodes = append(renderableNodes, node)
		}
	}

	// Cache storage nodes for future queries
	storageNodes := g.filterServiceNodes(renderableNodes, config.ServiceStorage)
	g.nodeCache.CacheServiceNodes(config.ServiceStorage, storageNodes)

	return renderableNodes
}

// processNodesInParallel processes nodes concurrently, returning the results
func (g *ArchDiagram) processNodesInParallel(allNodes []string) []*utils.NodeResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	totalNodes := len(allNodes)
	if totalNodes == 0 {
		return nil
	}

	workerCount := min(totalNodes, maxWorkers)
	jobCh := make(chan int, totalNodes)
	resultCh := make(chan *utils.NodeResult, totalNodes)
	var wg sync.WaitGroup

	var activeWorkers int32

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			atomic.AddInt32(&activeWorkers, 1)
			defer atomic.AddInt32(&activeWorkers, -1)

			for {
				select {
				case idx, ok := <-jobCh:
					if !ok {
						return
					}

					nodeName := allNodes[idx]
					isStorage := g.checkNodeInService(nodeName, config.ServiceStorage)

					nr := utils.GetNodeResult()
					nr.Index = idx
					nr.NodeName = nodeName
					nr.IsStorage = isStorage
					resultCh <- nr
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for i := 0; i < totalNodes; i++ {
		select {
		case jobCh <- i:
		case <-ctx.Done():
			logrus.Warn("Timeout while processing nodes")
			close(jobCh)
			goto collectResults
		}
	}
	close(jobCh)

collectResults:
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make([]*utils.NodeResult, 0, totalNodes)
	for nr := range resultCh {
		results = append(results, nr)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Index < results[j].Index
	})

	return results
}

// checkNodeInService checks if a node belongs to a specific service
func (g *ArchDiagram) checkNodeInService(
	nodeName string,
	serviceType config.ServiceType,
) bool {
	return g.isNodeInList(nodeName, g.getServiceNodes(serviceType))
}

// extractRenderableNodes extracts all nodes that should be rendered from the results
func (g *ArchDiagram) extractRenderableNodes(results []*utils.NodeResult) []string {
	renderableNodes := make([]string, 0, len(results))

	for _, r := range results {
		renderableNodes = append(renderableNodes, r.NodeName)
		utils.PutNodeResult(r)
	}

	return renderableNodes
}

// filterServiceNodes filters nodes by service type
func (g *ArchDiagram) filterServiceNodes(allNodes []string, serviceType config.ServiceType) []string {
	serviceNodes := g.getServiceNodes(serviceType)

	serviceNodeMap := make(map[string]struct{}, len(serviceNodes))
	for _, node := range serviceNodes {
		serviceNodeMap[node] = struct{}{}
	}

	filteredNodes := make([]string, 0, len(allNodes))
	for _, node := range allNodes {
		if _, exists := serviceNodeMap[node]; exists {
			filteredNodes = append(filteredNodes, node)
		}
	}

	return filteredNodes
}

// buildOrderedNodeList builds a list of nodes ordered by config appearance
func (g *ArchDiagram) buildOrderedNodeList() []string {
	nodeMap := g.getNodeMap()
	defer g.putNodeMap(nodeMap)

	g.mu.RLock()
	nodesLen := len(g.cfg.Nodes)
	g.mu.RUnlock()

	allNodes := make([]string, 0, nodesLen)

	g.mu.RLock()
	for _, node := range g.cfg.Nodes {
		if node.Host != "" && utils.IsIPAddress(node.Host) {
			if _, exists := nodeMap[node.Host]; !exists {
				nodeMap[node.Host] = struct{}{}
				allNodes = append(allNodes, node.Host)
			}
		}
	}

	groupIPsMap := make(map[string][]string, len(g.cfg.NodeGroups))

	for _, nodeGroup := range g.cfg.NodeGroups {
		ipList := g.expandNodeGroup(&nodeGroup)
		groupIPsMap[nodeGroup.Name] = ipList
	}

	for _, nodeGroup := range g.cfg.NodeGroups {
		ipList := groupIPsMap[nodeGroup.Name]

		for _, ip := range ipList {
			if _, exists := nodeMap[ip]; !exists {
				nodeMap[ip] = struct{}{}
				allNodes = append(allNodes, ip)
			}
		}
	}
	g.mu.RUnlock()

	return allNodes
}

// expandNodeGroup expands a node group into individual nodes
func (g *ArchDiagram) expandNodeGroup(nodeGroup *config.NodeGroup) []string {
	return g.nodeCache.GetNodeGroup(nodeGroup.Name, func() []string {
		if len(nodeGroup.Nodes) > 0 {
			ipList := make([]string, 0, len(nodeGroup.Nodes))
			for _, node := range nodeGroup.Nodes {
				if node.Host != "" && utils.IsIPAddress(node.Host) {
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
	})
}

// getServiceNodes returns nodes for a specific service type with caching
func (g *ArchDiagram) getServiceNodes(serviceType config.ServiceType) []string {
	return g.nodeCache.GetServiceNodes(serviceType, func() []string {
		nodes, nodeGroups := g.getServiceConfig(serviceType)
		result, err := g.getNodesForService(nodes, nodeGroups)
		if err != nil {
			logrus.WithError(err).Errorf("Failed to get nodes for service %s", serviceType)
			return nil
		}
		return result
	})
}

// getServiceConfig returns nodes and node groups for a service type
func (g *ArchDiagram) getServiceConfig(serviceType config.ServiceType) ([]string, []string) {
	g.mu.RLock()
	defer g.mu.RUnlock()

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

// getNodesForService returns nodes for a service with error handling
func (g *ArchDiagram) getNodesForService(nodes []string, nodeGroups []string) ([]string, error) {
	g.mu.RLock()
	cfg := g.cfg
	g.mu.RUnlock()

	if cfg == nil {
		return nil, NewConfigError("configuration is nil")
	}

	serviceNodes := make([]string, 0, len(nodes)+len(nodeGroups)*4)
	nodeMap := make(map[string]struct{}, len(nodes)+len(nodeGroups)*4)

	g.mu.RLock()
	for _, nodeName := range nodes {
		for _, node := range cfg.Nodes {
			if node.Name == nodeName {
				if node.Host != "" && utils.IsIPAddress(node.Host) {
					if _, exists := nodeMap[node.Host]; !exists {
						nodeMap[node.Host] = struct{}{}
						serviceNodes = append(serviceNodes, node.Host)
					}
				}
				break
			}
		}
	}

	nodeGroupMap := make(map[string]*config.NodeGroup, len(cfg.NodeGroups))
	for i := range cfg.NodeGroups {
		nodeGroup := &cfg.NodeGroups[i]
		nodeGroupMap[nodeGroup.Name] = nodeGroup
	}
	g.mu.RUnlock()

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

// getNodeServices returns the services running on a node
func (g *ArchDiagram) getNodeServices(node string) []string {
	services := make([]string, 0, len(serviceTypes))

	for _, svcType := range serviceTypes {
		if g.isNodeInList(node, g.getServiceNodes(svcType)) {
			displayName := serviceDisplayNames[svcType]
			services = append(services, fmt.Sprintf("[%s]", displayName))
		}
	}
	return services
}

// prepareServiceNodesMap prepares a map of service nodes
func (g *ArchDiagram) prepareServiceNodesMap(clientNodes []string) map[config.ServiceType][]string {
	g.mu.RLock()
	serviceConfigs := g.renderer.ServiceConfigs
	g.mu.RUnlock()

	serviceNodesMap := make(map[config.ServiceType][]string, len(serviceConfigs)+1)

	for _, cfg := range serviceConfigs {
		if cfg.Type == config.ServiceMeta {
			serviceNodesMap[cfg.Type] = g.getServiceNodes(config.ServiceMeta)
		} else {
			serviceNodesMap[cfg.Type] = g.getServiceNodes(cfg.Type)
		}
	}

	serviceNodesMap[config.ServiceClient] = clientNodes

	return serviceNodesMap
}

// countServiceNodes counts the number of nodes for each service type
func (g *ArchDiagram) countServiceNodes(serviceNodesMap map[config.ServiceType][]string) map[config.ServiceType]int {
	counts := make(map[config.ServiceType]int, len(serviceNodesMap))

	for svcType, nodes := range serviceNodesMap {
		counts[svcType] = len(nodes)
	}

	return counts
}

// ================ Utility Methods ================

// isNodeInList checks if a node is in a list using a map for O(1) lookup
func (g *ArchDiagram) isNodeInList(nodeName string, nodeList []string) bool {
	if len(nodeList) == 0 {
		return false
	}
	if len(nodeList) == 1 {
		return nodeList[0] == nodeName
	}

	return g.nodeCache.IsNodeInList(nodeName, nodeList)
}

// GetTotalNodeCount returns the total number of actual nodes
func (g *ArchDiagram) GetTotalNodeCount() int {
	g.mu.RLock()
	cfg := g.cfg
	g.mu.RUnlock()

	if cfg == nil {
		return 0
	}

	uniqueIPs := make(map[string]struct{}, initialMapCapacity)

	g.mu.RLock()
	for _, node := range cfg.Nodes {
		uniqueIPs[node.Host] = struct{}{}
	}

	for _, nodeGroup := range cfg.NodeGroups {
		ipList, err := utils.GenerateIPRange(nodeGroup.IPBegin, nodeGroup.IPEnd)
		if err != nil {
			continue
		}

		for _, ip := range ipList {
			uniqueIPs[ip] = struct{}{}
		}
	}
	g.mu.RUnlock()

	return len(uniqueIPs)
}

// ================ Object Pool Methods ================

// getNodeMap gets a map from the object pool
func (g *ArchDiagram) getNodeMap() map[string]struct{} {
	return make(map[string]struct{}, initialMapCapacity)
}

// putNodeMap returns a map to the object pool
func (g *ArchDiagram) putNodeMap(m map[string]struct{}) {
	for k := range m {
		delete(m, k)
	}
}
