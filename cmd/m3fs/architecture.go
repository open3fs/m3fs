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
)

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

// getCacheKey generates a unified format key for caching
func (g *ArchDiagram) getCacheKey(prefix string, parts ...string) string {
	if len(parts) == 0 {
		return prefix
	}
	return fmt.Sprintf("%s:%s", prefix, strings.Join(parts, ":"))
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

// ================ Object Pools ================

// nodeResultPool provides a reuse pool for nodeResult objects
var nodeResultPool = sync.Pool{
	New: func() any {
		return &nodeResult{}
	},
}

// ================ Core Struct and Methods ================

// ArchDiagram generates architecture diagrams for m3fs clusters
type ArchDiagram struct {
	cfg          *config.Config
	renderer     *render.DiagramRenderer
	cacheManager *cache.Manager

	// Concurrency control
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

	archDiagram := &ArchDiagram{
		cfg:          cfg,
		renderer:     render.NewDiagramRenderer(cfg),
		cacheManager: cache.NewCacheManager(cache.DefaultCacheConfig),
	}

	return archDiagram
}

// Generate generates an architecture diagram
func (g *ArchDiagram) Generate() string {
	if g.cfg == nil {
		return "Error: No configuration provided"
	}

	sb := &strings.Builder{}
	sb.Grow(1024)

	clientNodes := g.getServiceNodes(config.ServiceClient)
	storageNodes := g.getStorageRelatedNodes()
	serviceNodesMap := g.prepareServiceNodesMap(clientNodes)

	networkSpeed := g.getNetworkSpeed()

	g.renderer.RenderHeader(sb)
	g.renderClientSection(sb, clientNodes)
	g.renderNetworkSection(sb, networkSpeed)
	g.renderStorageSection(sb, storageNodes)
	g.renderSummarySection(sb, serviceNodesMap)

	return sb.String()
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

// getStorageRelatedNodes returns nodes that are related to storage services
func (g *ArchDiagram) getStorageRelatedNodes() []string {
	if g.cfg == nil {
		logrus.Error("Configuration is nil")
		return []string{"no storage node"}
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

	return storageNodes
}

// buildOrderedNodeList builds a list of nodes ordered by config appearance
func (g *ArchDiagram) buildOrderedNodeList() []string {
	allNodes := g.getNodeSlice()
	nodeMap := g.getNodeMap()
	defer func() {
		g.putNodeMap(nodeMap)
	}()

	// 1. Add individual node IP addresses
	for _, node := range g.cfg.Nodes {
		if node.Host != "" && g.isIPLike(node.Host) {
			if _, exists := nodeMap[node.Host]; !exists {
				nodeMap[node.Host] = struct{}{}
				allNodes = append(allNodes, node.Host)
			}
		}
	}

	// 2. Process each node group separately to ensure stable order
	groupIPsMap := make(map[string][]string)

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

	return allNodes
}

// processNodesInParallel processes nodes concurrently, returning the results
func (g *ArchDiagram) processNodesInParallel(allNodes []string) []*nodeResult {
	serviceNodesCache := &sync.Map{}
	resultChan := make(chan *nodeResult, len(allNodes))
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, maxWorkers)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for i, nodeName := range allNodes {
		wg.Add(1)
		go g.processNode(ctx, semaphore, &wg, resultChan, serviceNodesCache, nodeName, i)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	results := make([]*nodeResult, len(allNodes))
	for result := range resultChan {
		results[result.index] = result
	}

	return results
}

// processNode processes a single node to determine if it's a storage node
func (g *ArchDiagram) processNode(
	ctx context.Context,
	semaphore chan struct{},
	wg *sync.WaitGroup,
	resultChan chan<- *nodeResult,
	serviceNodesCache *sync.Map,
	name string,
	idx int,
) {
	defer wg.Done()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Recovered from panic in node processing: %v", r)
		}
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

	for _, svcType := range []config.ServiceType{
		config.ServiceStorage,
		config.ServiceFdb,
		config.ServiceMeta,
		config.ServiceMgmtd,
		config.ServiceMonitor,
		config.ServiceClickhouse,
	} {
		var serviceNodes []string
		if cached, ok := serviceNodesCache.Load(svcType); ok {
			serviceNodes = cached.([]string)
		} else {
			if svcType == config.ServiceMeta {
				serviceNodes = g.getMetaNodes()
			} else {
				serviceNodes = g.getServiceNodes(svcType)
			}
			serviceNodesCache.Store(svcType, serviceNodes)
		}

		if g.isNodeInList(name, serviceNodes) {
			result.isStorage = true
			resultChan <- result
			return
		}
	}

	resultChan <- result
}

// extractStorageNodes extracts storage nodes from the results
func (g *ArchDiagram) extractStorageNodes(results []*nodeResult) []string {
	storageNodes := g.getNodeSlice()

	for _, result := range results {
		if result != nil && result.isStorage {
			storageNodes = append(storageNodes, result.nodeName)
		}
		if result != nil {
			putNodeResult(result)
		}
	}

	return storageNodes
}

// getServiceNodes returns nodes for a specific service type with caching
func (g *ArchDiagram) getServiceNodes(serviceType config.ServiceType) []string {
	g.mu.RLock()
	if g.cfg == nil {
		g.mu.RUnlock()
		logrus.Error("Cannot get service nodes: configuration is nil")
		return []string{}
	}
	g.mu.RUnlock()

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
	if g.cfg == nil {
		return nil, &ConfigError{msg: "configuration is nil"}
	}

	serviceNodes := make([]string, 0, len(nodes)+len(nodeGroups))
	nodeMap := make(map[string]struct{}, len(nodes)+len(nodeGroups))

	// 1. Add individual node IP addresses
	for _, nodeName := range nodes {
		for _, node := range g.cfg.Nodes {
			if node.Name == nodeName {
				if node.Host != "" && g.isIPLike(node.Host) {
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
	nodeGroupMap := make(map[string]*config.NodeGroup)
	for i := range g.cfg.NodeGroups {
		nodeGroup := &g.cfg.NodeGroups[i]
		nodeGroupMap[nodeGroup.Name] = nodeGroup
	}

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

// isNodeInList checks if a node is in a list using a map for O(1) lookup
func (g *ArchDiagram) isNodeInList(nodeName string, nodeList []string) bool {
	if len(nodeList) == 0 {
		return false
	}

	if !g.cacheManager.Enabled {
		return g.checkNodeInListDirect(nodeName, nodeList)
	}

	key := g.getCacheKey("nodeinlist", nodeName, strings.Join(nodeList, ","))
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
	nodeSet := g.getNodeMap()
	defer g.putNodeMap(nodeSet)

	for _, n := range nodeList {
		nodeSet[n] = struct{}{}
	}
	_, exists := nodeSet[nodeName]
	return exists
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

// getClientNodes returns client nodes with caching
func (g *ArchDiagram) getClientNodes() []string {
	return g.getServiceNodes(config.ServiceClient)
}

// ================ Rendering Methods ================

// renderClientSection renders the client nodes section
func (g *ArchDiagram) renderClientSection(buffer *strings.Builder, clientNodes []string) {
	g.renderer.RenderSectionHeader(buffer, "CLIENT NODES:")

	clientCount := len(clientNodes)
	for i := 0; i < clientCount; i += g.renderer.RowSize {
		end := i + g.renderer.RowSize
		if end > clientCount {
			end = clientCount
		}
		g.renderClientNodes(buffer, clientNodes[i:end])
	}

	buffer.WriteByte('\n')
	g.renderArrows(buffer, g.calculateArrowCount(len(clientNodes)))
	buffer.WriteByte('\n')
}

// renderClientNodes renders client nodes
func (g *ArchDiagram) renderClientNodes(buffer *strings.Builder, clientNodes []string) {
	if len(clientNodes) == 0 {
		return
	}

	g.renderer.RenderNodesRow(buffer, clientNodes, func(node string) []string {
		return []string{"[hf3fs_fuse]"}
	})
}

// renderStorageSection renders the storage nodes section
func (g *ArchDiagram) renderStorageSection(buffer *strings.Builder, storageNodes []string) {
	g.renderArrows(buffer, g.calculateArrowCount(len(storageNodes)))
	buffer.WriteString("\n\n")

	g.renderer.RenderSectionHeader(buffer, "STORAGE NODES:")

	storageCount := len(storageNodes)
	for i := 0; i < storageCount; i += g.renderer.RowSize {
		g.renderStorageNodes(buffer, storageNodes[i:min(i+g.renderer.RowSize, storageCount)])
	}
}

// renderNetworkSection renders the network section
func (g *ArchDiagram) renderNetworkSection(buffer *strings.Builder, networkSpeed string) {
	networkText := fmt.Sprintf(" %s Network (%s) ", g.cfg.NetworkType, networkSpeed)
	rightPadding := g.renderer.Width - 2 - len(networkText)

	buffer.WriteString("╔" + strings.Repeat("═", g.renderer.Width-2) + "╗\n")
	buffer.WriteString("║")
	g.renderer.RenderWithColor(buffer, networkText, render.ColorBlue)
	buffer.WriteString(strings.Repeat(" ", rightPadding) + "║\n")
	buffer.WriteString("╚" + strings.Repeat("═", g.renderer.Width-2) + "╝\n")
}

// renderSummarySection renders the summary section
func (g *ArchDiagram) renderSummarySection(buffer *strings.Builder, serviceNodesMap map[config.ServiceType][]string) {
	buffer.WriteString("\n")
	g.renderer.RenderSectionHeader(buffer, "CLUSTER SUMMARY:")
	g.renderSummaryStatistics(buffer, serviceNodesMap)
}

// renderStorageNodes renders storage nodes
func (g *ArchDiagram) renderStorageNodes(buffer *strings.Builder, storageNodes []string) {
	if len(storageNodes) == 0 {
		return
	}

	g.renderer.RenderNodesRow(buffer, storageNodes, g.getNodeServices)
}

// getNodeServices returns the services running on a node
func (g *ArchDiagram) getNodeServices(node string) []string {
	// Define fixed order for service types
	orderedServiceTypes := []config.ServiceType{
		config.ServiceStorage,
		config.ServiceFdb,
		config.ServiceMeta,
		config.ServiceMgmtd,
		config.ServiceMonitor,
		config.ServiceClickhouse,
	}

	// Map service types to display names
	serviceNames := map[config.ServiceType]string{
		config.ServiceStorage:    "storage",
		config.ServiceFdb:        "foundationdb",
		config.ServiceMeta:       "meta",
		config.ServiceMgmtd:      "mgmtd",
		config.ServiceMonitor:    "monitor",
		config.ServiceClickhouse: "clickhouse",
	}

	// Check and add services in fixed order
	var services []string
	for _, svcType := range orderedServiceTypes {
		displayName := serviceNames[svcType]
		if g.isNodeInList(node, g.getServiceNodes(svcType)) {
			services = append(services, fmt.Sprintf("[%s]", displayName))
		}
	}
	return services
}

// calculateArrowCount calculates the number of arrows to display
func (g *ArchDiagram) calculateArrowCount(nodeCount int) int {
	if nodeCount <= 0 {
		return 1
	} else if nodeCount > 15 {
		return 15
	}
	return nodeCount
}

// renderArrows renders arrows for the specified count
func (g *ArchDiagram) renderArrows(buffer *strings.Builder, count int) {
	if count <= 0 {
		return
	}

	const arrowStr = "  ↓ "
	totalLen := len(arrowStr) * count

	arrowBuilder := g.renderer.GetStringBuilder()
	arrowBuilder.Grow(totalLen)

	for i := 0; i < count; i++ {
		arrowBuilder.WriteString(arrowStr)
	}

	buffer.WriteString(arrowBuilder.String())
	g.renderer.PutStringBuilder(arrowBuilder)
}

// renderSummaryStatistics renders the summary statistics
func (g *ArchDiagram) renderSummaryStatistics(
	buffer *strings.Builder,
	serviceNodesMap map[config.ServiceType][]string,
) {
	serviceNodeCounts := g.countServiceNodes(serviceNodesMap)
	totalNodeCount := g.getTotalActualNodeCount()

	// First row
	firstRow := []struct {
		name  string
		count int
		color string
	}{
		{"Client Nodes", serviceNodeCounts[config.ServiceClient], render.ColorGreen},
		{"Storage Nodes", serviceNodeCounts[config.ServiceStorage], render.ColorYellow},
		{"FoundationDB", serviceNodeCounts[config.ServiceFdb], render.ColorBlue},
		{"Meta Service", serviceNodeCounts[config.ServiceMeta], render.ColorPink},
	}

	for _, stat := range firstRow {
		buffer.WriteString(fmt.Sprintf("%s%-14s%s %-2d  ",
			g.renderer.GetColorCode(stat.color),
			stat.name+":",
			g.renderer.GetColorReset(),
			stat.count))
	}
	buffer.WriteByte('\n')

	// Second row
	secondRow := []struct {
		name  string
		count int
		color string
	}{
		{"Mgmtd Service", serviceNodeCounts[config.ServiceMgmtd], render.ColorPurple},
		{"Monitor Svc", serviceNodeCounts[config.ServiceMonitor], render.ColorPurple},
		{"Clickhouse", serviceNodeCounts[config.ServiceClickhouse], render.ColorRed},
		{"Total Nodes", totalNodeCount, render.ColorCyan},
	}

	for _, stat := range secondRow {
		buffer.WriteString(fmt.Sprintf("%s%-14s%s %-2d  ",
			g.renderer.GetColorCode(stat.color),
			stat.name+":",
			g.renderer.GetColorReset(),
			stat.count))
	}
	buffer.WriteByte('\n')
}

// countServiceNodes counts the nodes for each service type
func (g *ArchDiagram) countServiceNodes(
	serviceNodesMap map[config.ServiceType][]string,
) map[config.ServiceType]int {
	serviceNodeCounts := make(map[config.ServiceType]int, len(serviceNodesMap))

	for svcType, nodeList := range serviceNodesMap {
		uniqueIPs := g.countUniqueIPs(nodeList)
		serviceNodeCounts[svcType] = len(uniqueIPs)
	}

	return serviceNodeCounts
}

// countUniqueIPs counts unique IPs in a node list
func (g *ArchDiagram) countUniqueIPs(nodeList []string) map[string]struct{} {
	uniqueIPs := make(map[string]struct{}, len(nodeList))

	// Node list now contains only IP addresses, directly count them
	for _, ip := range nodeList {
		if g.isIPLike(ip) {
			uniqueIPs[ip] = struct{}{}
		}
	}

	return uniqueIPs
}

// ================ Network Methods ================

// getNetworkSpeed returns the network speed
func (g *ArchDiagram) getNetworkSpeed() string {
	return network.GetNetworkSpeed(string(g.cfg.NetworkType))
}

// ================ Utility Methods ================

// SetColorEnabled enables or disables color output in the diagram
func (g *ArchDiagram) SetColorEnabled(enabled bool) {
	g.renderer.SetColorEnabled(enabled)
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
	ipList, err := utils.GenerateIPRange(nodeGroup.IPBegin, nodeGroup.IPEnd)
	if err != nil {
		logrus.Errorf("Failed to expand node group %s: %v", nodeGroup.Name, err)
		return []string{}
	}

	return ipList
}

// isIPLike checks if a string looks like an IP address
func (g *ArchDiagram) isIPLike(s string) bool {
	ipPattern := regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
	return ipPattern.MatchString(s)
}

// getTotalActualNodeCount returns the total number of actual nodes
func (g *ArchDiagram) getTotalActualNodeCount() int {
	uniqueIPs := make(map[string]struct{})

	for _, node := range g.cfg.Nodes {
		uniqueIPs[node.Host] = struct{}{}
	}

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

// prepareServiceNodesMap prepares a map of service nodes
func (g *ArchDiagram) prepareServiceNodesMap(clientNodes []string) map[config.ServiceType][]string {
	serviceNodesMap := make(map[config.ServiceType][]string, len(g.renderer.ServiceConfigs)+1)

	for _, cfg := range g.renderer.ServiceConfigs {
		if cfg.Type == config.ServiceMeta {
			serviceNodesMap[cfg.Type] = g.getMetaNodes()
		} else {
			serviceNodesMap[cfg.Type] = g.getServiceNodes(cfg.Type)
		}
	}

	serviceNodesMap[config.ServiceClient] = clientNodes

	return serviceNodesMap
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
	return make([]string, 0, initialMapCapacity)
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
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
