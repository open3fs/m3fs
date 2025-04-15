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

/*
Package main implements architecture diagram generation for M3FS clusters.

Architecture Diagram Features:
- Generates ASCII art diagrams of M3FS cluster architectures with color formatting
- Visualizes client nodes, network connections, and storage-related services
- Displays network speed and connectivity information
- Produces cluster summary statistics

Core Components:
- ArchDiagram: Main struct for diagram generation
- NewArchDiagram: Creates and initializes the diagram generator
- Generate: Produces the complete architecture diagram

Configuration Management:
- setDefaultConfig: Sets default configuration values when not provided
- getDefaultServiceConfigs: Creates standard service configurations

Node Processing:
- getStorageRelatedNodes: Finds and processes all storage-related nodes
- buildOrderedNodeList: Creates an ordered list of nodes from configuration
- processNodesInParallel: Handles concurrent node processing with worker pools
- getServiceNodes: Retrieves nodes for specific service types
- getClientNodes: Gets client nodes with caching
- getMetaNodes: Gets meta nodes with caching
- isNodeInList: Efficiently checks if a node is in a list

Section Rendering:
- renderClusterHeader: Creates the cluster name header
- renderClientSection: Renders client nodes section
- renderNetworkSection: Displays network type and speed
- renderStorageSection: Shows storage nodes with their services
- renderSummarySection: Presents statistics summary of the cluster

Network Detection:
- getNetworkSpeed: Determines network speed from system
- getIBNetworkSpeed: Detects InfiniBand network speed
- getEthernetSpeed: Detects Ethernet network speed
- getDefaultInterface: Finds the default network interface

Memory Management:
- Object pools for strings.Builder, maps, slices, and processing results
- getStringBuilder/putStringBuilder: Manages string builder pool
- getNodeMap/putNodeMap: Manages node map pool
- getNodeSlice: Manages node slice pool
- getNodeResult/putNodeResult: Manages nodeResult pool

Cache Management:
- cacheServiceNodes: Caches service node information
- getCachedNodes: Retrieves cached node information
- startCacheCleanup: Initiates background cache cleanup
- cleanupCaches: Removes expired entries from caches

Error Handling:
- ConfigError: Represents configuration-related errors
- NetworkError: Represents network operation errors
- ServiceError: Represents service-related errors

Utility Functions:
- renderWithColor: Adds color to text output
- renderLine: Renders a text line with optional coloring
- renderDivider: Creates horizontal divider lines
- renderArrows: Creates arrow connectors between sections
- expandNodeGroup: Expands node groups into individual nodes
- getTotalActualNodeCount: Counts unique physical nodes
*/

package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/utils"
	"github.com/sirupsen/logrus"
)

// ================ Type Definitions and Constants ================

// Color and style constants
const (
	// Colors
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorPink   = "\033[38;5;219m"

	// Layout
	defaultRowSize           = 8
	defaultDiagramWidth      = 70
	defaultNodeCellWidth     = 16
	defaultServiceBoxPadding = 2
	defaultTotalCellWidth    = 14

	// Concurrency
	maxWorkers = 10
	timeout    = 30 * time.Second

	// Network
	networkTimeout = 5 * time.Second

	// Initial capacities
	initialStringBuilderCapacity = 1024
	initialMapCapacity           = 16
)

// NetworkType constants
const (
	NetworkTypeEthernet = "ethernet"
	NetworkTypeIB       = "ib"
	NetworkTypeRDMA     = "rdma"
)

// ServiceType defines the type of service in the 3fs cluster
type ServiceType string

// ServiceType constants
const (
	ServiceMgmtd      ServiceType = "mgmtd"
	ServiceMonitor    ServiceType = "monitor"
	ServiceStorage    ServiceType = "storage"
	ServiceFdb        ServiceType = "fdb"
	ServiceClickhouse ServiceType = "clickhouse"
	ServiceMeta       ServiceType = "meta"
	ServiceClient     ServiceType = "client"
)

// ServiceConfig defines a service configuration for rendering
type ServiceConfig struct {
	Type  ServiceType
	Name  string
	Color string
}

// StatInfo represents a statistic item in the summary
type StatInfo struct {
	Name  string
	Count int
	Color string
	Width int
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
	serviceType ServiceType
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

// Compile regex patterns once
var (
	ibSpeedPattern  = regexp.MustCompile(`rate:\s+(\d+)\s+Gb/sec`)
	ethSpeedPattern = regexp.MustCompile(`Speed:\s+(\d+)\s*([GMK]b/?s)`)
)

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

// Default cache configuration
var defaultCacheConfig = CacheConfig{
	TTL:             5 * time.Minute,
	CleanupInterval: 10 * time.Minute,
	Enabled:         true,
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
	colorEnabled bool

	// Layout constants
	defaultRowSize    int
	diagramWidth      int
	nodeCellWidth     int
	serviceBoxPadding int
	totalCellWidth    int

	// Service configurations
	serviceConfigs []ServiceConfig

	// Reusable buffers and caches
	stringBuilderPool sync.Pool
	serviceNodesCache sync.Map
	metaNodesCache    sync.Map
	nodeGroupCache    sync.Map
	mapPool           sync.Pool
	slicePool         sync.Pool

	// Cache management
	cacheConfig CacheConfig
	lastCleanup time.Time

	// Concurrency control
	mu sync.RWMutex
}

// NewArchDiagram creates a new ArchDiagram with default configuration
func NewArchDiagram(cfg *config.Config) *ArchDiagram {
	if cfg == nil {
		logrus.Warn("Creating ArchDiagram with nil config")
		cfg = &config.Config{
			Name:        "default",
			NetworkType: NetworkTypeEthernet,
		}
	}

	cfg = setDefaultConfig(cfg)

	archDiagram := &ArchDiagram{
		cfg:               cfg,
		colorEnabled:      true,
		defaultRowSize:    defaultRowSize,
		diagramWidth:      defaultDiagramWidth,
		nodeCellWidth:     defaultNodeCellWidth,
		serviceBoxPadding: defaultServiceBoxPadding,
		totalCellWidth:    defaultTotalCellWidth,
		serviceConfigs:    getDefaultServiceConfigs(),
		stringBuilderPool: sync.Pool{
			New: func() any {
				sb := &strings.Builder{}
				sb.Grow(initialStringBuilderCapacity)
				return sb
			},
		},
		// Add map and slice object pools
		mapPool: sync.Pool{
			New: func() any {
				return make(map[string]struct{}, initialMapCapacity)
			},
		},
		slicePool: sync.Pool{
			New: func() any {
				return make([]string, 0, initialMapCapacity)
			},
		},
		cacheConfig: defaultCacheConfig,
		lastCleanup: time.Now(),
	}

	if archDiagram.cacheConfig.Enabled && archDiagram.cacheConfig.CleanupInterval > 0 {
		go archDiagram.startCacheCleanup()
	}

	return archDiagram
}

// Generate generates an architecture diagram
func (g *ArchDiagram) Generate() string {
	if g.cfg == nil {
		return "Error: No configuration provided"
	}

	sb := g.getStringBuilder()
	defer g.putStringBuilder(sb)

	clientNodes := g.getServiceNodes(ServiceClient)
	storageNodes := g.getStorageRelatedNodes()
	serviceNodesMap := g.prepareServiceNodesMap(clientNodes)

	networkSpeed := g.getNetworkSpeed()

	g.renderClusterHeader(sb)
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
		cfg.NetworkType = NetworkTypeEthernet
	}
	return cfg
}

// getDefaultServiceConfigs returns default service configurations
func getDefaultServiceConfigs() []ServiceConfig {
	return []ServiceConfig{
		{ServiceStorage, "storage", colorYellow},
		{ServiceFdb, "foundationdb", colorBlue},
		{ServiceMeta, "meta", colorPink},
		{ServiceMgmtd, "mgmtd", colorPurple},
		{ServiceMonitor, "monitor", colorPurple},
		{ServiceClickhouse, "clickhouse", colorRed},
	}
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

	// Strictly display nodes in order: first individual node IPs, then IPs from each group in order
	
	// 1. Add individual node IP addresses
	for _, node := range g.cfg.Nodes {
		// Use the node's Host property (IP address)
		if node.Host != "" && g.isIPLike(node.Host) {
			if _, exists := nodeMap[node.Host]; !exists {
				nodeMap[node.Host] = struct{}{}
				allNodes = append(allNodes, node.Host)
			}
		}
	}

	// 2. Process each node group separately to ensure stable order
	// Map node group names to their internal IP lists
	groupIPsMap := make(map[string][]string)
	
	// Collect IP lists for all node groups
	for _, nodeGroup := range g.cfg.NodeGroups {
		ipList := g.expandNodeGroup(&nodeGroup)
		groupIPsMap[nodeGroup.Name] = ipList
	}
	
	// Iterate through node groups in definition order
	for _, nodeGroup := range g.cfg.NodeGroups {
		ipList := groupIPsMap[nodeGroup.Name]
		
		// Preserve internal order of IP list
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

	for _, svcConfig := range g.serviceConfigs {
		var serviceNodes []string
		if cached, ok := serviceNodesCache.Load(svcConfig.Type); ok {
			serviceNodes = cached.([]string)
		} else {
			if svcConfig.Type == ServiceMeta {
				serviceNodes = g.getMetaNodes()
			} else {
				serviceNodes = g.getServiceNodes(svcConfig.Type)
			}
			serviceNodesCache.Store(svcConfig.Type, serviceNodes)
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

// getCachedNodes retrieves nodes from cache
func (g *ArchDiagram) getCachedNodes(serviceType ServiceType) []string {
	if !g.cacheConfig.Enabled {
		return nil
	}

	if cached, ok := g.serviceNodesCache.Load(serviceType); ok {
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
func (g *ArchDiagram) cacheServiceNodes(serviceType ServiceType, nodes []string) {
	if !g.cacheConfig.Enabled {
		return
	}

	g.serviceNodesCache.Store(serviceType, cacheEntry{
		value:      nodes,
		expireTime: time.Now().Add(g.cacheConfig.TTL),
	})
}

// getServiceNodes returns nodes for a specific service type with caching
func (g *ArchDiagram) getServiceNodes(serviceType ServiceType) []string {
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

	if serviceType == ServiceClient && len(serviceNodes) == 0 {
		logrus.Debug("No client nodes found, using default-client")
		serviceNodes = []string{"default-client"}
	}

	g.cacheServiceNodes(serviceType, serviceNodes)
	return serviceNodes
}

// getServiceConfig returns nodes and node groups for a service type
func (g *ArchDiagram) getServiceConfig(serviceType ServiceType) ([]string, []string) {
	switch serviceType {
	case ServiceMgmtd:
		return g.cfg.Services.Mgmtd.Nodes, g.cfg.Services.Mgmtd.NodeGroups
	case ServiceMonitor:
		return g.cfg.Services.Monitor.Nodes, g.cfg.Services.Monitor.NodeGroups
	case ServiceStorage:
		return g.cfg.Services.Storage.Nodes, g.cfg.Services.Storage.NodeGroups
	case ServiceFdb:
		return g.cfg.Services.Fdb.Nodes, g.cfg.Services.Fdb.NodeGroups
	case ServiceClickhouse:
		return g.cfg.Services.Clickhouse.Nodes, g.cfg.Services.Clickhouse.NodeGroups
	case ServiceMeta:
		return g.cfg.Services.Meta.Nodes, g.cfg.Services.Meta.NodeGroups
	case ServiceClient:
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

	// Strictly retrieve nodes in order: first process individual nodes, then node groups
	
	// 1. Add individual node IP addresses
	for _, nodeName := range nodes {
		for _, node := range g.cfg.Nodes {
			if node.Name == nodeName {
				// Use the node's Host property
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
	for _, groupName := range nodeGroups {
		found := false
		for _, nodeGroup := range g.cfg.NodeGroups {
			if nodeGroup.Name == groupName {
				ipList := g.expandNodeGroup(&nodeGroup)
				for _, ip := range ipList {
					if _, exists := nodeMap[ip]; !exists {
						nodeMap[ip] = struct{}{}
						serviceNodes = append(serviceNodes, ip)
					}
				}
				found = true
				break
			}
		}
		if !found {
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

	if !g.cacheConfig.Enabled {
		return g.checkNodeInListDirect(nodeName, nodeList)
	}

	cacheKey := nodeName + ":" + strings.Join(nodeList, ",")
	if cached, ok := g.serviceNodesCache.Load(cacheKey); ok {
		if entry, ok := cached.(cacheEntry); ok {
			if time.Now().Before(entry.expireTime) {
				if result, ok := entry.value.(bool); ok {
					return result
				}
			}
		} else if result, ok := cached.(bool); ok {
			return result
		}
	}

	exists := g.checkNodeInListDirect(nodeName, nodeList)

	g.serviceNodesCache.Store(cacheKey, cacheEntry{
		value:      exists,
		expireTime: time.Now().Add(g.cacheConfig.TTL),
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
	if !g.cacheConfig.Enabled {
		return g.getServiceNodes(ServiceMeta)
	}

	if cached, ok := g.metaNodesCache.Load("meta"); ok {
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

	nodes := g.getServiceNodes(ServiceMeta)

	g.metaNodesCache.Store("meta", cacheEntry{
		value:      nodes,
		expireTime: time.Now().Add(g.cacheConfig.TTL),
	})

	return nodes
}

// getClientNodes returns client nodes with caching
func (g *ArchDiagram) getClientNodes() []string {
	return g.getServiceNodes(ServiceClient)
}

// ================ Rendering Methods ================

// renderClusterHeader renders the cluster header section
func (g *ArchDiagram) renderClusterHeader(buffer *strings.Builder) {
	g.renderLine(buffer, "Cluster: "+g.cfg.Name, "")
	g.renderDivider(buffer, "=", g.diagramWidth)
	buffer.WriteByte('\n')
}

// renderClientSection renders the client nodes section
func (g *ArchDiagram) renderClientSection(buffer *strings.Builder, clientNodes []string) {
	g.renderSectionHeader(buffer, "CLIENT NODES:")
	
	// Directly render client nodes instead of using renderNodeRow
	clientCount := len(clientNodes)
	for i := 0; i < clientCount; i += g.defaultRowSize {
		end := i + g.defaultRowSize
		if end > clientCount {
			end = clientCount
		}
		
		// Only render nodes within the current range
		g.renderClientNodes(buffer, clientNodes[i:end])
	}
	
	buffer.WriteByte('\n')

	arrowCount := g.calculateArrowCount(len(clientNodes))
	g.renderArrows(buffer, arrowCount)
	buffer.WriteByte('\n')
}

// renderStorageSection renders the storage nodes section
func (g *ArchDiagram) renderStorageSection(buffer *strings.Builder, storageNodes []string) {
	arrowCount := g.calculateArrowCount(len(storageNodes))
	g.renderArrows(buffer, arrowCount)
	buffer.WriteString("\n\n")

	g.renderSectionHeader(buffer, "STORAGE NODES:")
	
	// Directly render storage nodes to avoid duplication from renderNodeRow
	storageCount := len(storageNodes)
	for i := 0; i < storageCount; i += g.defaultRowSize {
		end := i + g.defaultRowSize
		if end > storageCount {
			end = storageCount
		}
		
		// Only render nodes within the current range
		g.renderStorageFunc(buffer, "", i)
	}
}

// renderNetworkSection renders the network section
func (g *ArchDiagram) renderNetworkSection(buffer *strings.Builder, networkSpeed string) {
	networkText := fmt.Sprintf(" %s Network (%s) ", g.cfg.NetworkType, networkSpeed)
	rightPadding := g.diagramWidth - 2 - len(networkText)

	buffer.WriteString("╔" + strings.Repeat("═", g.diagramWidth-2) + "╗\n")

	buffer.WriteString("║")
	g.renderWithColor(buffer, networkText, colorBlue)
	buffer.WriteString(strings.Repeat(" ", rightPadding) + "║\n")

	buffer.WriteString("╚" + strings.Repeat("═", g.diagramWidth-2) + "╝\n")
}

// renderSummarySection renders the summary section
func (g *ArchDiagram) renderSummarySection(buffer *strings.Builder, serviceNodesMap map[ServiceType][]string) {
	buffer.WriteString("\n")
	g.renderSectionHeader(buffer, "CLUSTER SUMMARY:")
	g.renderSummaryStatistics(buffer, serviceNodesMap)
}

// renderBoxBorder renders a box border with specified count
func (g *ArchDiagram) renderBoxBorder(buffer *strings.Builder, count int) {
	const borderPattern = "+----------------+ "
	g.renderNodeBoxRow(buffer, borderPattern, count)
}

// renderNodeBoxRow renders a row of node boxes
func (g *ArchDiagram) renderNodeBoxRow(buffer *strings.Builder, pattern string, count int) {
	for j := 0; j < count; j++ {
		buffer.WriteString(pattern)
	}
	buffer.WriteByte('\n')
}

// renderNodeRow renders a row of nodes
func (g *ArchDiagram) renderNodeRow(buffer *strings.Builder, nodes []string, rowSize int,
	renderFunc func(buffer *strings.Builder, nodeName string, index int)) {

	if len(nodes) == 0 {
		return
	}

	nodeCount := len(nodes)
	for i := 0; i < nodeCount; i += rowSize {
		end := g.calculateRowEndIndex(i, rowSize, nodeCount)

		g.renderBoxBorder(buffer, end-i)
		g.renderNodeNames(buffer, nodes, i, end)
		renderFunc(buffer, "", i)
		g.renderBoxBorder(buffer, end-i)
	}
}

// calculateRowEndIndex calculates the end index for a row
func (g *ArchDiagram) calculateRowEndIndex(startIndex, rowSize, totalCount int) int {
	end := startIndex + rowSize
	if end > totalCount {
		end = totalCount
	}
	return end
}

// renderNodeNames renders the node names
func (g *ArchDiagram) renderNodeNames(buffer *strings.Builder, nodes []string, startIndex, endIndex int) {
	for j := startIndex; j < endIndex; j++ {
		nodeName := g.formatNodeName(nodes[j])
		fmt.Fprintf(buffer, "|%s%-16s%s| ", g.getColorCode(colorCyan), nodeName, g.getColorReset())
	}
	buffer.WriteByte('\n')
}

// formatNodeName formats a node name, truncating if too long
func (g *ArchDiagram) formatNodeName(nodeName string) string {
	if len(nodeName) > g.nodeCellWidth {
		return nodeName[:13] + "..."
	}
	return nodeName
}

// renderServiceRow renders a row of services
func (g *ArchDiagram) renderServiceRow(buffer *strings.Builder,
	nodes []string, serviceNodes []string, startIndex int, endIndex int,
	serviceName string, color string) {

	// Simplify processing logic, directly render service label for each node
	// Since we have already specified the exact list of nodes to display services in the parameters, no need to check again
	for j := startIndex; j < endIndex; j++ {
		serviceLabel := "[" + serviceName + "]"
		paddingNeeded := g.totalCellWidth - len(serviceLabel)
		if paddingNeeded < 0 {
			paddingNeeded = 0
		}

		fmt.Fprintf(buffer, "|  %s%s%s%s| ",
			g.getColorCode(color),
			serviceLabel,
			g.getColorReset(),
			strings.Repeat(" ", paddingNeeded))
	}
	buffer.WriteByte('\n')
}

// renderStorageFunc renders storage nodes
func (g *ArchDiagram) renderStorageFunc(buffer *strings.Builder, _ string, startIndex int) {
	// Get all storage nodes
	storageNodes := g.getStorageRelatedNodes()

	// Calculate the range to display
	storageCount := len(storageNodes)
	endIndex := startIndex + g.defaultRowSize
	if endIndex > storageCount {
		endIndex = storageCount
	}

	// Only render nodes within the current row range
	currentNodes := storageNodes[startIndex:endIndex]
	nodeCount := len(currentNodes)

	// Define layout format
	const (
		cellContent = "                "
		boxSpacing  = " "
	)

	// Prepare service nodes for efficient lookup
	serviceMap := make(map[ServiceType][]string)
	for _, cfg := range g.serviceConfigs {
		var serviceNodes []string
		if cfg.Type == ServiceMeta {
			serviceNodes = g.getMetaNodes()
		} else {
			serviceNodes = g.getServiceNodes(cfg.Type)
		}
		serviceMap[cfg.Type] = serviceNodes
	}

	// Render top border for all nodes
	topBorderLine := ""
	for i := 0; i < nodeCount; i++ {
		topBorderLine += "+" + strings.Repeat("-", len(cellContent)) + "+"
		if i < nodeCount-1 {
			topBorderLine += boxSpacing
		}
	}
	buffer.WriteString(topBorderLine + "\n")

	// Render node name row
	nodeNameLine := ""
	for _, node := range currentNodes {
		nodeName := g.formatNodeName(node)
		nodeNameLine += "|" + fmt.Sprintf("%s%-16s%s", g.getColorCode(colorCyan), nodeName, g.getColorReset()) + "|"
		if nodeNameLine != topBorderLine { // If not the last node
			nodeNameLine += boxSpacing
		}
	}
	buffer.WriteString(nodeNameLine + "\n")

	// Define service configurations
	serviceConfigs := []struct {
		Type  ServiceType
		Name  string
		Color string
	}{
		{ServiceStorage, "storage", colorYellow},
		{ServiceFdb, "foundationdb", colorBlue},
		{ServiceMeta, "meta", colorPink},
		{ServiceMgmtd, "mgmtd", colorPurple},
		{ServiceMonitor, "monitor", colorPurple},
		{ServiceClickhouse, "clickhouse", colorRed},
	}

	// Render each service row
	for _, cfg := range serviceConfigs {
		serviceNodes := serviceMap[cfg.Type]
		serviceLabel := "[" + cfg.Name + "]"
		padding := len(cellContent) - len(serviceLabel) - 2 // -2 for the leading spaces

		serviceLine := ""
		for _, node := range currentNodes {
			hasService := g.isNodeInList(node, serviceNodes)
			if hasService {
				serviceLine += "|" + fmt.Sprintf("  %s%s%s%s", 
					g.getColorCode(cfg.Color),
					serviceLabel,
					g.getColorReset(),
					strings.Repeat(" ", padding)) + "|"
			} else {
				serviceLine += "|" + cellContent + "|"
			}
			if serviceLine != topBorderLine { // If not the last node
				serviceLine += boxSpacing
			}
		}
		buffer.WriteString(serviceLine + "\n")
	}

	// Render bottom border for all nodes
	bottomBorderLine := ""
	for i := 0; i < nodeCount; i++ {
		bottomBorderLine += "+" + strings.Repeat("-", len(cellContent)) + "+"
		if i < nodeCount-1 {
			bottomBorderLine += boxSpacing
		}
	}
	buffer.WriteString(bottomBorderLine + "\n")
}

// renderClientNodes renders client nodes with independent borders
func (g *ArchDiagram) renderClientNodes(buffer *strings.Builder, clientNodes []string) {
	if len(clientNodes) == 0 {
		return
	}

	nodeCount := len(clientNodes)
	
	// Define layout format
	const (
		cellContent = "                "
		boxSpacing  = " "
	)

	// Render top border for all nodes
	topBorderLine := ""
	for i := 0; i < nodeCount; i++ {
		topBorderLine += "+" + strings.Repeat("-", len(cellContent)) + "+"
		if i < nodeCount-1 {
			topBorderLine += boxSpacing
		}
	}
	buffer.WriteString(topBorderLine + "\n")

	// Render node name row
	nodeNameLine := ""
	for _, node := range clientNodes {
		nodeName := g.formatNodeName(node)
		nodeNameLine += "|" + fmt.Sprintf("%s%-16s%s", g.getColorCode(colorCyan), nodeName, g.getColorReset()) + "|"
		if nodeNameLine != topBorderLine { // If not the last node
			nodeNameLine += boxSpacing
		}
	}
	buffer.WriteString(nodeNameLine + "\n")

	// Render client service row
	serviceLine := ""
	for range clientNodes {
		serviceLabel := "[hf3fs_fuse]"
		padding := len(cellContent) - len(serviceLabel) - 2 // -2 for the leading spaces
		serviceLine += "|" + fmt.Sprintf("  %s%s%s%s", 
			g.getColorCode(colorGreen),
			serviceLabel,
			g.getColorReset(),
			strings.Repeat(" ", padding)) + "|"
		if serviceLine != topBorderLine { // If not the last node
			serviceLine += boxSpacing
		}
	}
	buffer.WriteString(serviceLine + "\n")

	// Render bottom border for all nodes
	bottomBorderLine := ""
	for i := 0; i < nodeCount; i++ {
		bottomBorderLine += "+" + strings.Repeat("-", len(cellContent)) + "+"
		if i < nodeCount-1 {
			bottomBorderLine += boxSpacing
		}
	}
	buffer.WriteString(bottomBorderLine + "\n")
}

// renderClientFunc renders client nodes
func (g *ArchDiagram) renderClientFunc(buffer *strings.Builder, _ string, startIndex int) {
	clientNodes := g.getClientNodes()
	if len(clientNodes) == 0 {
		return
	}

	clientCount := len(clientNodes)
	endIndex := startIndex + g.defaultRowSize
	if endIndex > clientCount {
		endIndex = clientCount
	}

	// Ensure correct node and service alignment
	nodes := clientNodes[startIndex:endIndex]
	g.renderServiceRow(buffer, nodes, clientNodes, 0, len(nodes), "hf3fs_fuse", colorGreen)
}

// renderSectionHeader renders a section header
func (g *ArchDiagram) renderSectionHeader(buffer *strings.Builder, title string) {
	g.renderWithColor(buffer, title, colorCyan)
	buffer.WriteByte('\n')
	g.renderDivider(buffer, "-", g.diagramWidth)
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

// renderSummaryRow renders a row in the summary section
func (g *ArchDiagram) renderSummaryRow(buffer *strings.Builder, stats []StatInfo) {
	for _, stat := range stats {
		buffer.WriteString(fmt.Sprintf("%s%-"+fmt.Sprintf("%d", stat.Width)+"s%s %-2d  ",
			g.getColorCode(stat.Color), stat.Name+":", g.getColorReset(), stat.Count))
	}
	buffer.WriteString("\n")
}

// renderSummaryStatistics renders the summary statistics
func (g *ArchDiagram) renderSummaryStatistics(buffer *strings.Builder, serviceNodesMap map[ServiceType][]string) {
	nodeHosts := g.buildNodeHostsMap()
	serviceNodeCounts := g.countServiceNodes(serviceNodesMap, nodeHosts)
	totalNodeCount := g.getTotalActualNodeCount()
	g.renderSummaryRows(buffer, serviceNodeCounts, totalNodeCount)
}

// buildNodeHostsMap builds a map of node names to hosts
func (g *ArchDiagram) buildNodeHostsMap() map[string]string {
	nodeHosts := make(map[string]string, len(g.cfg.Nodes))
	for _, node := range g.cfg.Nodes {
		nodeHosts[node.Name] = node.Host
	}
	return nodeHosts
}

// countServiceNodes counts the nodes for each service type
func (g *ArchDiagram) countServiceNodes(
	serviceNodesMap map[ServiceType][]string,
	nodeHosts map[string]string,
) map[ServiceType]int {
	serviceNodeCounts := make(map[ServiceType]int, len(serviceNodesMap))

	for svcType, nodeList := range serviceNodesMap {
		uniqueIPs := g.countUniqueIPs(nodeList, nodeHosts)
		serviceNodeCounts[svcType] = len(uniqueIPs)
	}

	return serviceNodeCounts
}

// countUniqueIPs counts unique IPs in a node list
func (g *ArchDiagram) countUniqueIPs(nodeList []string, nodeHosts map[string]string) map[string]struct{} {
	uniqueIPs := make(map[string]struct{}, len(nodeList))

	// Node list now contains only IP addresses, directly count them
	for _, ip := range nodeList {
		if g.isIPLike(ip) {
			uniqueIPs[ip] = struct{}{}
		}
	}

	return uniqueIPs
}

// processNodeGroupIfPresent checks if a node is part of any node group's IP range
func (g *ArchDiagram) processNodeGroupIfPresent(nodeName string, uniqueIPs map[string]struct{}) bool {
	// Check if the node name is an IP that's part of any node group's IP range
	for _, nodeGroup := range g.cfg.NodeGroups {
		ipList, err := utils.GenerateIPRange(nodeGroup.IPBegin, nodeGroup.IPEnd)
		if err != nil {
			logrus.Errorf("Failed to expand node group %s: %v", nodeGroup.Name, err)
			continue
		}

		// Check if the node name is in the expanded IP list
		for _, ip := range ipList {
			if ip == nodeName {
				uniqueIPs[nodeName] = struct{}{}
				return true
			}
		}
	}
	return false
}

// processIndividualNode processes an individual node
func (g *ArchDiagram) processIndividualNode(
	nodeName string,
	nodeHosts map[string]string,
	uniqueIPs map[string]struct{},
) {
	if host, ok := nodeHosts[nodeName]; ok {
		uniqueIPs[host] = struct{}{}
	} else if nodeName == "default-client" || nodeName == "no storage node" {
		uniqueIPs[nodeName] = struct{}{}
	}
}

// renderSummaryRows renders the summary rows
func (g *ArchDiagram) renderSummaryRows(
	buffer *strings.Builder,
	serviceNodeCounts map[ServiceType]int,
	totalNodeCount int,
) {
	firstRowStats := []StatInfo{
		{Name: "Client Nodes", Count: serviceNodeCounts[ServiceClient], Color: colorGreen, Width: 13},
		{Name: "Storage Nodes", Count: serviceNodeCounts[ServiceStorage], Color: colorYellow, Width: 14},
		{Name: "FoundationDB", Count: serviceNodeCounts[ServiceFdb], Color: colorBlue, Width: 12},
		{Name: "Meta Service", Count: serviceNodeCounts[ServiceMeta], Color: colorPink, Width: 12},
	}
	g.renderSummaryRow(buffer, firstRowStats)

	secondRowStats := []StatInfo{
		{Name: "Mgmtd Service", Count: serviceNodeCounts[ServiceMgmtd], Color: colorPurple, Width: 13},
		{Name: "Monitor Svc", Count: serviceNodeCounts[ServiceMonitor], Color: colorPurple, Width: 14},
		{Name: "Clickhouse", Count: serviceNodeCounts[ServiceClickhouse], Color: colorRed, Width: 12},
		{Name: "Total Nodes", Count: totalNodeCount, Color: colorCyan, Width: 12},
	}
	g.renderSummaryRow(buffer, secondRowStats)
}

// ================ Network Methods ================

// getNetworkSpeed returns the network speed
func (g *ArchDiagram) getNetworkSpeed() string {
	if speed := g.getIBNetworkSpeed(); speed != "" {
		return speed
	}

	if speed := g.getEthernetSpeed(); speed != "" {
		return speed
	}

	return g.getDefaultNetworkSpeed()
}

// getDefaultNetworkSpeed returns the default network speed
func (g *ArchDiagram) getDefaultNetworkSpeed() string {
	switch g.cfg.NetworkType {
	case config.NetworkTypeIB:
		return "50 Gb/sec"
	case config.NetworkTypeRDMA:
		return "100 Gb/sec"
	default:
		return "10 Gb/sec"
	}
}

// getIBNetworkSpeed returns the InfiniBand network speed
func (g *ArchDiagram) getIBNetworkSpeed() string {
	ctx, cancel := context.WithTimeout(context.Background(), networkTimeout)
	defer cancel()

	cmdIB := exec.CommandContext(ctx, "ibstatus")
	output, err := cmdIB.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logrus.Error("Timeout while getting IB network speed")
		} else {
			logrus.Debugf("Failed to get IB network speed: %v", err)
		}
		return ""
	}

	matches := ibSpeedPattern.FindStringSubmatch(string(output))
	if len(matches) > 1 {
		return matches[1] + " Gb/sec"
	}

	logrus.Debug("No IB network speed found in ibstatus output")
	return ""
}

// getEthernetSpeed returns the Ethernet network speed
func (g *ArchDiagram) getEthernetSpeed() string {
	interfaceName, err := g.getDefaultInterface()
	if err != nil {
		logrus.Debugf("Failed to get default interface: %v", err)
		return ""
	}
	if interfaceName == "" {
		logrus.Debug("No default interface found")
		return ""
	}

	speed := g.getInterfaceSpeed(interfaceName)
	if speed == "" {
		logrus.Debugf("Failed to get speed for interface %s", interfaceName)
	}
	return speed
}

// getDefaultInterface returns the default network interface
func (g *ArchDiagram) getDefaultInterface() (string, error) {
	cmdIp := exec.Command("sh", "-c", "ip route | grep default | awk '{print $5}'")
	interfaceOutput, err := cmdIp.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get default interface: %w", err)
	}

	interfaceName := strings.TrimSpace(string(interfaceOutput))
	if interfaceName == "" {
		return "", fmt.Errorf("no default interface found")
	}
	return interfaceName, nil
}

// getInterfaceSpeed returns the speed of a network interface
func (g *ArchDiagram) getInterfaceSpeed(interfaceName string) string {
	if interfaceName == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), networkTimeout)
	defer cancel()

	cmdEthtool := exec.CommandContext(ctx, "ethtool", interfaceName)
	ethtoolOutput, err := cmdEthtool.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logrus.Error("Timeout while getting interface speed")
		} else {
			logrus.Debugf("Failed to get interface speed for %s: %v", interfaceName, err)
		}
		return ""
	}

	matches := ethSpeedPattern.FindStringSubmatch(string(ethtoolOutput))
	if len(matches) > 2 {
		return matches[1] + " " + matches[2]
	}

	logrus.Debugf("No speed found in ethtool output for interface %s", interfaceName)
	return ""
}

// ================ Utility Methods ================

// SetColorEnabled enables or disables color output in the diagram
func (g *ArchDiagram) SetColorEnabled(enabled bool) {
	g.colorEnabled = enabled
}

// getColorReset returns the color reset code
func (g *ArchDiagram) getColorReset() string {
	return g.getColorCode(colorReset)
}

// getColorCode returns a color code if colors are enabled
func (g *ArchDiagram) getColorCode(colorCode string) string {
	if !g.colorEnabled {
		return ""
	}
	return colorCode
}

// getStringBuilder gets a strings.Builder from the pool
func (g *ArchDiagram) getStringBuilder() *strings.Builder {
	return g.stringBuilderPool.Get().(*strings.Builder)
}

// putStringBuilder returns a strings.Builder to the pool
func (g *ArchDiagram) putStringBuilder(sb *strings.Builder) {
	sb.Reset()
	g.stringBuilderPool.Put(sb)
}

// expandNodeGroup expands a node group into individual nodes
func (g *ArchDiagram) expandNodeGroup(nodeGroup *config.NodeGroup) []string {
	if !g.cacheConfig.Enabled {
		return g.expandNodeGroupDirect(nodeGroup)
	}

	cacheKey := fmt.Sprintf("%s[%s-%s]", nodeGroup.Name, nodeGroup.IPBegin, nodeGroup.IPEnd)

	if cached, ok := g.nodeGroupCache.Load(cacheKey); ok {
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

	g.nodeGroupCache.Store(cacheKey, cacheEntry{
		value:      nodes,
		expireTime: time.Now().Add(g.cacheConfig.TTL),
	})

	return nodes
}

// expandNodeGroupDirect directly expands a node group without caching
func (g *ArchDiagram) expandNodeGroupDirect(nodeGroup *config.NodeGroup) []string {
	// Instead of returning the node group name with IP range, actually expand the IP range
	ipList, err := utils.GenerateIPRange(nodeGroup.IPBegin, nodeGroup.IPEnd)
	if err != nil {
		logrus.Errorf("Failed to expand node group %s: %v", nodeGroup.Name, err)
		return []string{}
	}

	return ipList
}

// isIPLike checks if a string looks like an IP address
func (g *ArchDiagram) isIPLike(s string) bool {
	// Simple check for IP address format (contains dots and numbers in correct pattern)
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
func (g *ArchDiagram) prepareServiceNodesMap(clientNodes []string) map[ServiceType][]string {
	serviceNodesMap := make(map[ServiceType][]string, len(g.serviceConfigs)+1)

	for _, cfg := range g.serviceConfigs {
		if cfg.Type == ServiceMeta {
			serviceNodesMap[cfg.Type] = g.getMetaNodes()
		} else {
			serviceNodesMap[cfg.Type] = g.getServiceNodes(cfg.Type)
		}
	}

	serviceNodesMap[ServiceClient] = clientNodes

	return serviceNodesMap
}

// renderWithColor renders text with the specified color
func (g *ArchDiagram) renderWithColor(buffer *strings.Builder, text string, color string) {
	if !g.colorEnabled || color == "" {
		buffer.WriteString(text)
		return
	}

	totalLen := len(color) + len(text) + len(colorReset)

	sb := g.getStringBuilder()
	sb.Grow(totalLen)

	sb.WriteString(color)
	sb.WriteString(text)
	sb.WriteString(colorReset)

	buffer.WriteString(sb.String())

	g.putStringBuilder(sb)
}

// renderLine renders a line of text, optionally with color
func (g *ArchDiagram) renderLine(buffer *strings.Builder, text string, color string) {
	if color != "" {
		g.renderWithColor(buffer, text, color)
	} else {
		buffer.WriteString(text)
	}
	buffer.WriteByte('\n')
}

// renderDivider renders a divider line
func (g *ArchDiagram) renderDivider(buffer *strings.Builder, char string, width int) {
	if width <= 0 {
		return
	}

	divBuilder := g.getStringBuilder()
	divBuilder.Grow(width + 1)

	divBuilder.WriteString(strings.Repeat(char, width))
	divBuilder.WriteByte('\n')

	buffer.WriteString(divBuilder.String())
	g.putStringBuilder(divBuilder)
}

// renderArrows renders arrows for the specified count
func (g *ArchDiagram) renderArrows(buffer *strings.Builder, count int) {
	if count <= 0 {
		return
	}

	const arrowStr = "  ↓ "
	totalLen := len(arrowStr) * count

	arrowBuilder := g.getStringBuilder()
	arrowBuilder.Grow(totalLen)

	for i := 0; i < count; i++ {
		arrowBuilder.WriteString(arrowStr)
	}

	buffer.WriteString(arrowBuilder.String())
	g.putStringBuilder(arrowBuilder)
}

// getNodeMap gets a map from the object pool
func (g *ArchDiagram) getNodeMap() map[string]struct{} {
	if v := g.mapPool.Get(); v != nil {
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
	g.mapPool.Put(m)
}

// getNodeSlice gets a slice from the object pool
func (g *ArchDiagram) getNodeSlice() []string {
	if v := g.slicePool.Get(); v != nil {
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

// startCacheCleanup starts the cache cleanup routine
func (g *ArchDiagram) startCacheCleanup() {
	ticker := time.NewTicker(g.cacheConfig.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		g.cleanupCaches()
	}
}

// cleanupCaches cleans up expired cache entries
func (g *ArchDiagram) cleanupCaches() {
	now := time.Now()
	g.lastCleanup = now

	g.cleanupCache(&g.serviceNodesCache, now)
	g.cleanupCache(&g.metaNodesCache, now)
	g.cleanupCache(&g.nodeGroupCache, now)

	logrus.Debugf("Cache cleanup completed at %v", now)
}

// cleanupCache cleans up a specific cache
func (g *ArchDiagram) cleanupCache(cache *sync.Map, now time.Time) {
	var keysToDelete []any

	cache.Range(func(key, value any) bool {
		if entry, ok := value.(cacheEntry); ok {
			if entry.expireTime.Before(now) {
				keysToDelete = append(keysToDelete, key)
			}
		} else {
			keysToDelete = append(keysToDelete, key)
		}
		return true
	})

	for _, key := range keysToDelete {
		cache.Delete(key)
	}
}
