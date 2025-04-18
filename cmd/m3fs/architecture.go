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
	"regexp"
	"strings"
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
var (
	// Compiled regex for IP validation - only compile once
	ipPattern = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
)

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

// cacheEntry represents a cached value with metadata
type cacheEntry struct {
	value      any
	expireTime time.Time
}

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

// stringBuilderPool provides a reuse pool for strings.Builder objects
var stringBuilderPool = sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

// ================ Core Struct and Methods ================

// ArchDiagram generates architecture diagrams for m3fs clusters
type ArchDiagram struct {
	cfg          *config.Config
	renderer     *render.DiagramRenderer
	archRenderer *render.ArchDiagramRenderer
	cacheManager *cache.Manager

	// Concurrency control with RWMutex for better read concurrency
	mu sync.RWMutex
}

// Ensure ArchDiagram implements render.NodeDataProvider
var _ render.NodeDataProvider = (*ArchDiagram)(nil)

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

	// Check cache first for storage nodes
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

	// Cache the storage nodes for future use
	g.cacheStorageNodes(storageNodes)
	return storageNodes
}

// getCachedStorageNodes retrieves storage nodes from cache
func (g *ArchDiagram) getCachedStorageNodes() []string {
	if !g.cacheManager.Enabled {
		return nil
	}

	key := "storage_nodes"
	if cached, ok := g.renderer.ServiceNodesCache.Load(key); ok {
		if entry, ok := cached.(cacheEntry); ok {
			if time.Now().Before(entry.expireTime) {
				if nodes, ok := entry.value.([]string); ok {
					return nodes
				}
			}
		} else if nodes, ok := cached.([]string); ok {
			return nodes
		}
	}
	return nil
}

// cacheStorageNodes caches storage nodes
func (g *ArchDiagram) cacheStorageNodes(nodes []string) {
	if !g.cacheManager.Enabled {
		return
	}

	key := "storage_nodes"
	g.renderer.ServiceNodesCache.Store(key, cacheEntry{
		value:      nodes,
		expireTime: time.Now().Add(g.cacheManager.TTL),
	})
}

// buildOrderedNodeList builds a list of nodes ordered by config appearance
func (g *ArchDiagram) buildOrderedNodeList() []string {
	allNodes := g.getNodeSlice()
	nodeMap := g.getNodeMap()
	defer func() {
		g.putNodeMap(nodeMap)
	}()

	// Pre-allocate for known capacity
	g.mu.RLock()
	nodesLen := len(g.cfg.Nodes)
	g.mu.RUnlock()

	if cap(allNodes) < nodesLen {
		allNodes = make([]string, 0, nodesLen)
	}

	// 1. Add individual node IP addresses
	g.mu.RLock()
	for _, node := range g.cfg.Nodes {
		if node.Host != "" && ipPattern.MatchString(node.Host) {
			if _, exists := nodeMap[node.Host]; !exists {
				nodeMap[node.Host] = struct{}{}
				allNodes = append(allNodes, node.Host)
			}
		}
	}

	// 2. Process each node group separately to ensure stable order
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
	resultChan := make(chan *nodeResult, len(allNodes))
	var wg sync.WaitGroup

	// Semaphore to limit concurrent goroutines
	semaphore := make(chan struct{}, maxWorkers)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Track how many nodes are processed
	var processedCount int32

	// Process nodes in batches
	for i, nodeName := range allNodes {
		wg.Add(1)
		go func(idx int, name string) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("Recovered from panic in node processing: %v", r)
				}
				atomic.AddInt32(&processedCount, 1)
			}()

			// Acquire semaphore or return if context is done
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

			// Check if the node belongs to any of the relevant services
			for _, svcType := range serviceTypes {
				if g.checkNodeInService(name, svcType, serviceNodesCache) {
					result.isStorage = true
					resultChan <- result
					return
				}
			}

			resultChan <- result
		}(i, nodeName)
	}

	// Wait for all goroutines to finish or timeout
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	results := make([]*nodeResult, len(allNodes))
	for result := range resultChan {
		results[result.index] = result
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
		serviceNodesCache.Store(cacheKey, serviceNodes)
	}

	return g.isNodeInList(nodeName, serviceNodes)
}

// extractStorageNodes extracts storage nodes from the results
func (g *ArchDiagram) extractStorageNodes(results []*nodeResult) []string {
	// Pre-allocate with capacity hint based on results length
	storageNodes := make([]string, 0, len(results)/2)
	storageNodesMap := make(map[string]struct{}, len(results)/2)

	for _, result := range results {
		if result != nil && result.isStorage {
			// Deduplicate nodes
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

	// Pre-allocate with capacity hint
	serviceNodes := make([]string, 0, len(nodes)+len(nodeGroups)*4)
	nodeMap := make(map[string]struct{}, len(nodes)+len(nodeGroups)*4)

	g.mu.RLock()
	// 1. Add individual node IP addresses
	for _, nodeName := range nodes {
		for _, node := range cfg.Nodes {
			if node.Name == nodeName {
				if node.Host != "" && ipPattern.MatchString(node.Host) {
					if _, exists := nodeMap[node.Host]; !exists {
						nodeMap[node.Host] = struct{}{}
						serviceNodes = append(serviceNodes, node.Host)
					}
				}
				break
			}
		}
	}

	// 2. Add IP addresses from each node group in order
	// Build nodeGroup name to configuration object mapping
	nodeGroupMap := make(map[string]*config.NodeGroup, len(cfg.NodeGroups))
	for i := range cfg.NodeGroups {
		nodeGroup := &cfg.NodeGroups[i]
		nodeGroupMap[nodeGroup.Name] = nodeGroup
	}
	g.mu.RUnlock()

	// Process nodeGroups list in order to ensure stable ordering
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
	// Pre-allocate with capacity hint based on number of service types
	services := make([]string, 0, len(serviceTypes))

	// Check and add services in fixed order
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

// getCacheKey generates a unified format key for caching
func (g *ArchDiagram) getCacheKey(prefix string, parts ...string) string {
	if len(parts) == 0 {
		return prefix
	}

	// Use string builder from pool to reduce allocations
	sb := getStringBuilder()
	defer putStringBuilder(sb)

	sb.WriteString(prefix)
	sb.WriteByte(':')

	for i, part := range parts {
		if i > 0 {
			sb.WriteByte(':')
		}
		sb.WriteString(part)
	}

	return sb.String()
}

// getCachedNodes retrieves nodes from cache
func (g *ArchDiagram) getCachedNodes(serviceType config.ServiceType) []string {
	if !g.cacheManager.Enabled {
		return nil
	}

	key := g.getCacheKey("service", string(serviceType))
	if cached, ok := g.renderer.ServiceNodesCache.Load(key); ok {
		if entry, ok := cached.(cacheEntry); ok {
			if time.Now().Before(entry.expireTime) {
				if nodes, ok := entry.value.([]string); ok {
					return nodes
				}
			}
		} else if nodes, ok := cached.([]string); ok {
			return nodes
		}
	}
	return nil
}

// cacheServiceNodes caches service nodes
func (g *ArchDiagram) cacheServiceNodes(serviceType config.ServiceType, nodes []string) {
	if !g.cacheManager.Enabled {
		return
	}

	key := g.getCacheKey("service", string(serviceType))
	g.renderer.ServiceNodesCache.Store(key, cacheEntry{
		value:      nodes,
		expireTime: time.Now().Add(g.cacheManager.TTL),
	})
}

// isNodeInList checks if a node is in a list using a map for O(1) lookup
func (g *ArchDiagram) isNodeInList(nodeName string, nodeList []string) bool {
	if len(nodeList) == 0 {
		return false
	}

	if !g.cacheManager.Enabled {
		return g.checkNodeInListDirect(nodeName, nodeList)
	}

	key := g.getCacheKey("nodeinlist", nodeName, fmt.Sprintf("%d", len(nodeList)))
	if cached, ok := g.renderer.ServiceNodesCache.Load(key); ok {
		if entry, ok := cached.(cacheEntry); ok {
			if time.Now().Before(entry.expireTime) {
				if exists, ok := entry.value.(bool); ok {
					return exists
				}
			}
		} else if exists, ok := cached.(bool); ok {
			return exists
		}
	}

	exists := g.checkNodeInListDirect(nodeName, nodeList)
	g.renderer.ServiceNodesCache.Store(key, cacheEntry{
		value:      exists,
		expireTime: time.Now().Add(g.cacheManager.TTL),
	})

	return exists
}

// checkNodeInListDirect directly checks if a node is in list without using cache
func (g *ArchDiagram) checkNodeInListDirect(nodeName string, nodeList []string) bool {
	// Quick path: check direct matches first before allocating a map
	for _, n := range nodeList {
		if n == nodeName {
			return true
		}
	}

	// If list is large, switch to map-based lookup for better performance
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

// getMetaNodes returns meta nodes with caching
func (g *ArchDiagram) getMetaNodes() []string {
	if !g.cacheManager.Enabled {
		return g.getServiceNodes(config.ServiceMeta)
	}

	key := g.getCacheKey("meta")
	if cached, ok := g.renderer.MetaNodesCache.Load(key); ok {
		if entry, ok := cached.(cacheEntry); ok {
			if time.Now().Before(entry.expireTime) {
				if nodes, ok := entry.value.([]string); ok {
					return nodes
				}
			}
		} else if nodes, ok := cached.([]string); ok {
			return nodes
		}
	}

	nodes := g.getServiceNodes(config.ServiceMeta)
	g.renderer.MetaNodesCache.Store(key, cacheEntry{
		value:      nodes,
		expireTime: time.Now().Add(g.cacheManager.TTL),
	})

	return nodes
}

// expandNodeGroup expands a node group into individual nodes
func (g *ArchDiagram) expandNodeGroup(nodeGroup *config.NodeGroup) []string {
	if !g.cacheManager.Enabled {
		return g.expandNodeGroupDirect(nodeGroup)
	}

	key := g.getCacheKey("nodegroup", nodeGroup.Name, nodeGroup.IPBegin, nodeGroup.IPEnd)
	if cached, ok := g.renderer.NodeGroupCache.Load(key); ok {
		if entry, ok := cached.(cacheEntry); ok {
			if time.Now().Before(entry.expireTime) {
				if nodes, ok := entry.value.([]string); ok {
					return nodes
				}
			}
		} else if nodes, ok := cached.([]string); ok {
			return nodes
		}
	}

	nodes := g.expandNodeGroupDirect(nodeGroup)
	g.renderer.NodeGroupCache.Store(key, cacheEntry{
		value:      nodes,
		expireTime: time.Now().Add(g.cacheManager.TTL),
	})

	return nodes
}

// expandNodeGroupDirect directly expands a node group without caching
func (g *ArchDiagram) expandNodeGroupDirect(nodeGroup *config.NodeGroup) []string {
	// If Nodes slice is already populated, extract IP addresses from it
	if len(nodeGroup.Nodes) > 0 {
		// Pre-allocate to avoid growing the slice
		ipList := make([]string, 0, len(nodeGroup.Nodes))
		for _, node := range nodeGroup.Nodes {
			if node.Host != "" && ipPattern.MatchString(node.Host) {
				ipList = append(ipList, node.Host)
			}
		}
		if len(ipList) > 0 {
			return ipList
		}
	}

	// Fall back to generating IP range if Nodes not populated
	ipList, err := utils.GenerateIPRange(nodeGroup.IPBegin, nodeGroup.IPEnd)
	if err != nil {
		logrus.Errorf("Failed to expand node group %s: %v", nodeGroup.Name, err)
		return []string{}
	}

	return ipList
}

// ================ Network Methods ================

// getNetworkSpeed returns the network speed
func (g *ArchDiagram) getNetworkSpeed() string {
	g.mu.RLock()
	networkType := g.cfg.NetworkType
	g.mu.RUnlock()

	return network.GetNetworkSpeed(string(networkType))
}

// ================ Utility Methods ================

// SetColorEnabled enables or disables color output in the diagram
func (g *ArchDiagram) SetColorEnabled(enabled bool) {
	g.renderer.SetColorEnabled(enabled)
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

// getNodeSlice gets a slice from the object pool
func (g *ArchDiagram) getNodeSlice() []string {
	if v := g.renderer.NodeSlicePool.Get(); v != nil {
		s := v.([]string)
		return s[:0]
	}
	return make([]string, 0, initialSliceCapacity)
}

// getStringBuilder gets a strings.Builder from the object pool
func getStringBuilder() *strings.Builder {
	sb := stringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// putStringBuilder returns a strings.Builder to the object pool
func putStringBuilder(sb *strings.Builder) {
	stringBuilderPool.Put(sb)
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

// min returns the minimum of two integers
//
//nolint:unused // This function is kept for backward compatibility
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
