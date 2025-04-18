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
	"sync"
	"sync/atomic"
	"time"

	"github.com/open3fs/m3fs/pkg/cache"
	"github.com/open3fs/m3fs/pkg/config"
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

// Regex patterns
var ()

// Service display names - use a map for constant-time lookup
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

// ConfigError represents a configuration-related error
type ConfigError struct {
	msg string
}

func (e *ConfigError) Error() string {
	return e.msg
}

// NetworkError represents a network-related error with operation context
type NetworkError struct {
	operation string
	err       error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("%s failed: %v", e.operation, e.err)
}

// ServiceError represents a service-related error with service type context
type ServiceError struct {
	serviceType config.ServiceType
	err         error
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("service %s error: %v", e.serviceType, e.err)
}

// nodeResult represents the result of node processing
type nodeResult struct {
	index     int
	nodeName  string
	isStorage bool
}

// ================ Caching Types ================

// CacheConfig defines cache configuration parameters
type CacheConfig struct {
	// Time-to-live for cache entries
	TTL time.Duration
	// Interval for cache cleanup operations
	CleanupInterval time.Duration
	// Whether caching is enabled
	Enabled bool
}

// ================ Object Pools ================

// nodeResultPool provides a reuse pool for nodeResult objects
var nodeResultPool = sync.Pool{
	New: func() any {
		return &nodeResult{}
	},
}

// ================ Core Struct ================

// ArchDiagram generates architecture diagrams for m3fs clusters
type ArchDiagram struct {
	cfg          *config.Config
	renderer     *render.DiagramRenderer
	archRenderer *render.ArchDiagramRenderer
	cacheManager *cache.Manager

	mu                sync.RWMutex
	serviceNodesCache sync.Map
}

// Ensure ArchDiagram implements render.NodeDataProvider
var _ render.NodeDataProvider = (*ArchDiagram)(nil)

// ================ Constructor and Core Methods ================

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
		cfg:          cfg,
		renderer:     baseRenderer,
		archRenderer: render.NewArchDiagramRenderer(baseRenderer),
		cacheManager: cache.NewCacheManager(cache.DefaultCacheConfig),
	}

	return archDiagram
}

// Generate generates an architecture diagram
func (g *ArchDiagram) Generate() string {
	if g.cfg == nil {
		return "Error: No configuration provided"
	}

	adapter := render.NewArchDiagramAdapter(g, g.archRenderer)
	return adapter.Generate()
}

// SetColorEnabled enables or disables color output in the diagram
func (g *ArchDiagram) SetColorEnabled(enabled bool) {
	g.renderer.SetColorEnabled(enabled)
}

// ================ Configuration Methods ================

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

// GetClientNodes implements render.NodeDataProvider
func (g *ArchDiagram) GetClientNodes() []string {
	return g.getServiceNodes(config.ServiceClient)
}

// GetRenderableNodes implements render.NodeDataProvider
func (g *ArchDiagram) GetRenderableNodes() []string {
	return g.getRenderableNodes()
}

// GetNodeServices implements render.NodeDataProvider
func (g *ArchDiagram) GetNodeServices(nodeName string) []string {
	return g.getNodeServices(nodeName)
}

// GetServiceNodeCounts implements render.NodeDataProvider
func (g *ArchDiagram) GetServiceNodeCounts() map[config.ServiceType]int {
	clientNodes := g.GetClientNodes()
	serviceNodesMap := g.prepareServiceNodesMap(clientNodes)
	return g.countServiceNodes(serviceNodesMap)
}

// GetTotalNodeCount implements render.NodeDataProvider
func (g *ArchDiagram) GetTotalNodeCount() int {
	return g.getTotalActualNodeCount()
}

// GetNetworkSpeed implements render.NodeDataProvider
func (g *ArchDiagram) GetNetworkSpeed() string {
	return g.getNetworkSpeed()
}

// GetNetworkType implements render.NodeDataProvider
func (g *ArchDiagram) GetNetworkType() string {
	if g.cfg == nil {
		return "ethernet"
	}
	return string(g.cfg.NetworkType)
}

// ================ Node Processing Methods ================

// getRenderableNodes returns nodes that need to be rendered in the architecture diagram
func (g *ArchDiagram) getRenderableNodes() []string {
	g.mu.RLock()
	cfg := g.cfg
	g.mu.RUnlock()

	if cfg == nil {
		logrus.Error("Configuration is nil")
		return []string{"no storage node"}
	}

	if nodes := g.getCachedStorageNodes(); nodes != nil {
		return nodes
	}

	allNodes := g.buildOrderedNodeList()
	if len(allNodes) == 0 {
		logrus.Warn("No nodes found in configuration")
		return []string{"no storage node"}
	}

	results := g.processNodesInParallel(allNodes)
	storageNodes := g.extractStorageNodes(results)

	if len(storageNodes) == 0 {
		logrus.Warn("No storage nodes found")
		return []string{"no storage node"}
	}

	g.cacheStorageNodes(storageNodes)
	return storageNodes
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

// processNodesInParallel processes nodes concurrently, returning the results
func (g *ArchDiagram) processNodesInParallel(allNodes []string) []*nodeResult {
	serviceNodesCache := &sync.Map{}
	resultChan := make(chan *nodeResult, utils.Min(len(allNodes), maxWorkers*2))
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, maxWorkers)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var processedCount int32

	for i := 0; i < len(allNodes); i += maxWorkers {
		end := utils.Min(i+maxWorkers, len(allNodes))
		batch := allNodes[i:end]

		for j, nodeName := range batch {
			idx := i + j
			wg.Add(1)
			go func(idx int, name string) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						logrus.Errorf("Recovered from panic in node processing: %v", r)
					}
					atomic.AddInt32(&processedCount, 1)
				}()

				select {
				case semaphore <- struct{}{}:
					defer func() { <-semaphore }()
				case <-ctx.Done():
					logrus.Errorf("Timeout while processing node %s", name)
					return
				}

				result := getNodeResult()
				result.index = idx
				result.nodeName = name
				result.isStorage = false

				for _, svcType := range serviceTypes {
					if g.checkNodeInService(name, svcType, serviceNodesCache) {
						result.isStorage = true
						resultChan <- result
						return
					}
				}

				resultChan <- result
			}(idx, nodeName)
		}
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	results := make([]*nodeResult, len(allNodes))
	for result := range resultChan {
		if result != nil && result.index >= 0 && result.index < len(results) {
			results[result.index] = result
		}
	}

	return results
}

// checkNodeInService checks if a node belongs to a specific service
func (g *ArchDiagram) checkNodeInService(
	nodeName string,
	svcType config.ServiceType,
	serviceNodesCache *sync.Map,
) bool {
	cacheKey := string(svcType)
	var serviceNodes []string

	if cached, ok := serviceNodesCache.Load(cacheKey); ok {
		serviceNodes = cached.([]string)
	} else {
		if svcType == config.ServiceMeta {
			serviceNodes = g.getMetaNodes()
		} else {
			serviceNodes = g.getServiceNodes(svcType)
		}

		if len(serviceNodes) > 0 {
			serviceNodesCache.Store(cacheKey, serviceNodes)
		}
	}

	return g.isNodeInList(nodeName, serviceNodes)
}

// extractStorageNodes extracts storage nodes from the results
func (g *ArchDiagram) extractStorageNodes(results []*nodeResult) []string {
	storageNodes := make([]string, 0, len(results)/2)
	storageNodesMap := make(map[string]struct{}, len(results)/2)

	for _, result := range results {
		if result != nil && result.isStorage {
			if _, exists := storageNodesMap[result.nodeName]; !exists {
				storageNodesMap[result.nodeName] = struct{}{}
				storageNodes = append(storageNodes, result.nodeName)
			}
		}
		if result != nil {
			putNodeResult(result)
		}
	}

	return storageNodes
}

// getTotalActualNodeCount returns the total number of actual nodes
func (g *ArchDiagram) getTotalActualNodeCount() int {
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

// ================ Service Methods ================

// getServiceNodes returns nodes for a specific service type with caching
func (g *ArchDiagram) getServiceNodes(serviceType config.ServiceType) []string {
	g.mu.RLock()
	cfg := g.cfg
	g.mu.RUnlock()

	if cfg == nil {
		logrus.Error("Cannot get service nodes: configuration is nil")
		return []string{}
	}

	if nodes := g.getCachedNodes(serviceType); nodes != nil {
		return nodes
	}

	nodes, nodeGroups := g.getServiceConfig(serviceType)
	serviceNodes, err := g.getNodesForService(nodes, nodeGroups)
	if err != nil {
		logrus.Errorf("Failed to get nodes for service %s: %v", serviceType, err)
		return []string{}
	}

	if serviceType == config.ServiceClient && len(serviceNodes) == 0 {
		logrus.Debug("No client nodes found")
	}

	g.cacheServiceNodes(serviceType, serviceNodes)
	return serviceNodes
}

// getMetaNodes returns meta nodes with caching
func (g *ArchDiagram) getMetaNodes() []string {
	if !g.cacheManager.Enabled {
		return g.getServiceNodes(config.ServiceMeta)
	}

	key := "meta"
	if nodes := g.cacheManager.GetCachedNodes(&g.renderer.MetaNodesCache, key); nodes != nil {
		return nodes
	}

	nodes := g.getServiceNodes(config.ServiceMeta)
	g.cacheManager.CacheNodes(&g.renderer.MetaNodesCache, key, nodes)
	return nodes
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
		return nil, &ConfigError{msg: "configuration is nil"}
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
			serviceNodesMap[cfg.Type] = g.getMetaNodes()
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

// ================ Cache Methods ================

// getCachedStorageNodes retrieves storage nodes from cache
func (g *ArchDiagram) getCachedStorageNodes() []string {
	if !g.cacheManager.Enabled {
		return nil
	}

	return g.cacheManager.GetCachedNodes(&g.renderer.ServiceNodesCache, "storage_nodes")
}

// cacheStorageNodes caches storage nodes
func (g *ArchDiagram) cacheStorageNodes(nodes []string) {
	if !g.cacheManager.Enabled || len(nodes) == 0 {
		return
	}

	g.cacheManager.CacheNodes(&g.renderer.ServiceNodesCache, "storage_nodes", nodes)
}

// getCachedNodes retrieves nodes from cache
func (g *ArchDiagram) getCachedNodes(serviceType config.ServiceType) []string {
	if !g.cacheManager.Enabled {
		return nil
	}

	if cached, ok := g.serviceNodesCache.Load(serviceType); ok {
		return cached.([]string)
	}

	key := g.cacheManager.GetCacheKey("service", string(serviceType))
	if nodes := g.cacheManager.GetCachedNodes(&g.renderer.ServiceNodesCache, key); nodes != nil {
		g.serviceNodesCache.Store(serviceType, nodes)
		return nodes
	}

	return nil
}

// cacheServiceNodes caches service nodes
func (g *ArchDiagram) cacheServiceNodes(serviceType config.ServiceType, nodes []string) {
	if !g.cacheManager.Enabled || len(nodes) == 0 {
		return
	}

	g.serviceNodesCache.Store(serviceType, nodes)

	key := g.cacheManager.GetCacheKey("service", string(serviceType))
	g.cacheManager.CacheNodes(&g.renderer.ServiceNodesCache, key, nodes)
}

// ================ Node Group Methods ================

// expandNodeGroup expands a node group into individual nodes
func (g *ArchDiagram) expandNodeGroup(nodeGroup *config.NodeGroup) []string {
	if !g.cacheManager.Enabled {
		return g.expandNodeGroupDirect(nodeGroup)
	}

	key := g.cacheManager.GetCacheKey("nodegroup", nodeGroup.Name, nodeGroup.IPBegin, nodeGroup.IPEnd)
	if nodes := g.cacheManager.GetCachedNodes(&g.renderer.NodeGroupCache, key); nodes != nil {
		return nodes
	}

	nodes := g.expandNodeGroupDirect(nodeGroup)
	g.cacheManager.CacheNodes(&g.renderer.NodeGroupCache, key, nodes)

	return nodes
}

// expandNodeGroupDirect directly expands a node group without caching
func (g *ArchDiagram) expandNodeGroupDirect(nodeGroup *config.NodeGroup) []string {
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

	if !g.cacheManager.Enabled {
		return g.checkNodeInListDirect(nodeName, nodeList)
	}

	key := g.cacheManager.GetCacheKey("nodeinlist", nodeName, fmt.Sprintf("%d", len(nodeList)))
	nodes := g.cacheManager.GetCachedNodes(&g.renderer.ServiceNodesCache, key)
	if nodes != nil {
		return len(nodes) > 0
	}

	exists := g.checkNodeInListDirect(nodeName, nodeList)
	var value []string
	if exists {
		value = []string{"exists"}
	} else {
		value = []string{}
	}
	g.cacheManager.CacheNodes(&g.renderer.ServiceNodesCache, key, value)

	return exists
}

// checkNodeInListDirect directly checks if a node is in list without using cache
func (g *ArchDiagram) checkNodeInListDirect(nodeName string, nodeList []string) bool {
	for _, n := range nodeList {
		if n == nodeName {
			return true
		}
	}

	if len(nodeList) > 10 {
		nodeSet := g.getNodeMap()
		defer g.putNodeMap(nodeSet)

		for _, n := range nodeList {
			nodeSet[n] = struct{}{}
		}
		_, exists := nodeSet[nodeName]
		return exists
	}

	return false
}

// ================ Network Methods ================

// getNetworkSpeed returns the network speed
func (g *ArchDiagram) getNetworkSpeed() string {
	g.mu.RLock()
	networkType := g.cfg.NetworkType
	g.mu.RUnlock()

	return network.GetNetworkSpeed(string(networkType))
}

// ================ Object Pool Methods ================

// getNodeMap gets a map from the object pool
func (g *ArchDiagram) getNodeMap() map[string]struct{} {
	if v := g.renderer.NodeMapPool.Get(); v != nil {
		m := v.(map[string]struct{})
		for k := range m {
			delete(m, k)
		}
		return m
	}
	return make(map[string]struct{}, initialMapCapacity)
}

// putNodeMap returns a map to the object pool
func (g *ArchDiagram) putNodeMap(m map[string]struct{}) {
	g.renderer.NodeMapPool.Put(m)
}

// getNodeResult gets a nodeResult from the object pool
func getNodeResult() *nodeResult {
	return nodeResultPool.Get().(*nodeResult)
}

// putNodeResult returns a nodeResult to the object pool
func putNodeResult(nr *nodeResult) {
	nr.index = 0
	nr.nodeName = ""
	nr.isStorage = false
	nodeResultPool.Put(nr)
}
