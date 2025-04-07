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
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/utils"
)

// Color related constants
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorPink   = "\033[38;5;219m" // Light pink color
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
}

// NewArchDiagram creates a new ArchDiagram
func NewArchDiagram(cfg *config.Config) *ArchDiagram {
	return &ArchDiagram{
		cfg:               cfg,
		colorEnabled:      true,
		defaultRowSize:    8,
		diagramWidth:      70,
		nodeCellWidth:     16,
		serviceBoxPadding: 2,
		totalCellWidth:    14,
	}
}

// SetColorEnabled enables or disables colored output in the diagram
func (g *ArchDiagram) SetColorEnabled(enabled bool) {
	g.colorEnabled = enabled
}

// Generate generates an architecture diagram
func (g *ArchDiagram) Generate() string {
	// Prepare buffer for diagram output
	var buffer bytes.Buffer

	// Collect node information
	clientNodes := g.getServiceNodes(ServiceClient)
	storageNodes := g.getStorageNodes()
	serviceNodesMap := g.prepareServiceNodesMap(clientNodes)

	// Get network speed
	networkSpeed := g.getNetworkSpeed()

	// Generate diagram sections
	g.renderClusterHeader(&buffer)
	g.renderClientSection(&buffer, clientNodes)
	g.renderNetworkSection(&buffer, networkSpeed)
	g.renderStorageSection(&buffer, storageNodes)
	g.renderSummarySection(&buffer, serviceNodesMap)

	return buffer.String()
}

// getColorReset returns the color reset code if colors are enabled
func (g *ArchDiagram) getColorReset() string {
	return g.getColorCode(colorReset)
}

// renderBoxBorder renders a border line for node boxes
func (g *ArchDiagram) renderBoxBorder(buffer *bytes.Buffer, count int) {
	for j := 0; j < count; j++ {
		buffer.WriteString("+----------------+ ")
	}
	buffer.WriteByte('\n')
}

// renderNodeRow renders a row of nodes with the given style
func (g *ArchDiagram) renderNodeRow(buffer *bytes.Buffer, nodes []string, rowSize int,
	renderFunc func(buffer *bytes.Buffer, nodeName string, index int)) {

	if len(nodes) == 0 {
		return
	}

	nodeCount := len(nodes)
	for i := 0; i < nodeCount; i += rowSize {
		end := i + rowSize
		if end > nodeCount {
			end = nodeCount
		}

		// Render top border
		g.renderBoxBorder(buffer, end-i)

		// Render node names
		for j := i; j < end; j++ {
			nodeName := nodes[j]
			if len(nodeName) > g.nodeCellWidth {
				nodeName = nodeName[:13] + "..."
			}
			fmt.Fprintf(buffer, "|%s%-16s%s| ", g.getColorCode(colorCyan), nodeName, g.getColorReset())
		}
		buffer.WriteByte('\n')

		// Render service details
		renderFunc(buffer, "", i)

		// Render bottom border
		g.renderBoxBorder(buffer, end-i)
	}
}

// renderServiceRow renders a row showing a specific service on the nodes
func (g *ArchDiagram) renderServiceRow(buffer *bytes.Buffer,
	nodes []string, serviceNodes []string, startIndex int, endIndex int,
	serviceName string, color string) {

	for j := startIndex; j < endIndex; j++ {
		nodeName := nodes[j]
		if g.isNodeInList(nodeName, serviceNodes) {
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
		} else {
			buffer.WriteString("|                | ")
		}
	}
	buffer.WriteByte('\n')
}

// ServiceConfig defines a service configuration for rendering
type ServiceConfig struct {
	Type  ServiceType
	Name  string
	Color string
}

// Common service configs for rendering
var (
	serviceConfigs = []ServiceConfig{
		{ServiceStorage, "storage", colorYellow},
		{ServiceFdb, "foundationdb", colorBlue},
		{ServiceMeta, "meta", colorPink},
		{ServiceMgmtd, "mgmtd", colorPurple},
		{ServiceMonitor, "monitor", colorPurple},
		{ServiceClickhouse, "clickhouse", colorRed},
	}
)

// renderStorageFunc renders all services for storage nodes
func (g *ArchDiagram) renderStorageFunc(buffer *bytes.Buffer, _ string, startIndex int) {
	storageNodes := g.getStorageNodes()

	storageCount := len(storageNodes)
	endIndex := startIndex + g.defaultRowSize
	if endIndex > storageCount {
		endIndex = storageCount
	}

	for _, cfg := range serviceConfigs {
		var nodes []string
		if cfg.Type == ServiceMeta {
			nodes = g.getMetaNodes() // Special case for meta nodes
		} else {
			nodes = g.getServiceNodes(cfg.Type)
		}
		g.renderServiceRow(buffer, storageNodes, nodes, startIndex, endIndex, cfg.Name, cfg.Color)
	}
}

// StatInfo represents a statistic item in the summary
type StatInfo struct {
	Name  string
	Count int
	Color string
	Width int
}

// renderSummaryRow renders a row of statistics
func (g *ArchDiagram) renderSummaryRow(buffer *bytes.Buffer, stats []StatInfo) {
	for _, stat := range stats {
		buffer.WriteString(fmt.Sprintf("%s%-"+fmt.Sprintf("%d", stat.Width)+"s%s %-2d  ",
			g.getColorCode(stat.Color), stat.Name+":", g.getColorReset(), stat.Count))
	}
	buffer.WriteString("\n")
}

// renderClientFunc renders client services
func (g *ArchDiagram) renderClientFunc(buffer *bytes.Buffer, _ string, startIndex int) {
	clientNodes := g.getServiceNodes(ServiceClient)
	if len(clientNodes) == 0 {
		return
	}

	clientCount := len(clientNodes)
	endIndex := startIndex + g.defaultRowSize
	if endIndex > clientCount {
		endIndex = clientCount
	}

	g.renderServiceRow(buffer, clientNodes, clientNodes, startIndex, endIndex, "hf3fs_fuse", colorGreen)
}

// prepareServiceNodesMap prepares a map of service types to their respective nodes
func (g *ArchDiagram) prepareServiceNodesMap(clientNodes []string) map[ServiceType][]string {
	serviceNodesMap := make(map[ServiceType][]string, len(serviceConfigs)+1)

	// Fill the map with standard services
	for _, cfg := range serviceConfigs {
		if cfg.Type == ServiceMeta {
			serviceNodesMap[cfg.Type] = g.getMetaNodes()
		} else {
			serviceNodesMap[cfg.Type] = g.getServiceNodes(cfg.Type)
		}
	}

	// Add client nodes to the map
	serviceNodesMap[ServiceClient] = clientNodes

	return serviceNodesMap
}

// renderClusterHeader renders the cluster header section
func (g *ArchDiagram) renderClusterHeader(buffer *bytes.Buffer) {
	fmt.Fprintf(buffer, "Cluster: %s\n%s\n\n",
		g.cfg.Name,
		strings.Repeat("=", g.diagramWidth))
}

// renderSectionHeader renders a section header with the given title
func (g *ArchDiagram) renderSectionHeader(buffer *bytes.Buffer, title string) {
	fmt.Fprintf(buffer, "%s%s%s\n%s\n",
		g.getColorCode(colorCyan),
		title,
		g.getColorReset(),
		strings.Repeat("-", g.diagramWidth))
}

// renderClientSection renders the client nodes section
func (g *ArchDiagram) renderClientSection(buffer *bytes.Buffer, clientNodes []string) {
	g.renderSectionHeader(buffer, "CLIENT NODES:")

	g.renderNodeRow(buffer, clientNodes, g.defaultRowSize, g.renderClientFunc)
	buffer.WriteByte('\n')

	// Calculate appropriate arrow count
	arrowCount := g.calculateArrowCount(len(clientNodes))
	buffer.WriteString(strings.Repeat("  ↓ ", arrowCount))
	buffer.WriteByte('\n')
}

// renderStorageSection renders the storage nodes section
func (g *ArchDiagram) renderStorageSection(buffer *bytes.Buffer, storageNodes []string) {
	// Calculate storage arrow count
	arrowCount := g.calculateArrowCount(len(storageNodes))
	buffer.WriteString(strings.Repeat("  ↓ ", arrowCount))
	buffer.WriteString("\n\n")

	// Storage nodes section
	g.renderSectionHeader(buffer, "STORAGE NODES:")

	g.renderNodeRow(buffer, storageNodes, g.defaultRowSize, g.renderStorageFunc)
}

// renderSummarySection renders the cluster summary section
func (g *ArchDiagram) renderSummarySection(buffer *bytes.Buffer,
	serviceNodesMap map[ServiceType][]string) {

	buffer.WriteString("\n")
	g.renderSectionHeader(buffer, "CLUSTER SUMMARY:")

	// Render summary statistics in two rows for better readability
	g.renderSummaryStatistics(buffer, serviceNodesMap)
}

// calculateArrowCount calculates appropriate arrow count for display
func (g *ArchDiagram) calculateArrowCount(nodeCount int) int {
	if nodeCount <= 0 {
		return 1
	} else if nodeCount > 15 {
		return 15
	}
	return nodeCount
}

// renderNetworkSection renders the network section of the diagram
func (g *ArchDiagram) renderNetworkSection(buffer *bytes.Buffer, networkSpeed string) {
	networkText := fmt.Sprintf(" %s Network (%s) ", g.cfg.NetworkType, networkSpeed)
	rightPadding := g.diagramWidth - 2 - len(networkText)

	buffer.WriteString("╔" + strings.Repeat("═", g.diagramWidth-2) + "╗\n")
	fmt.Fprintf(buffer, "║%s%s%s%s║\n",
		g.getColorCode(colorBlue),
		networkText,
		g.getColorReset(),
		strings.Repeat(" ", rightPadding))
	buffer.WriteString("╚" + strings.Repeat("═", g.diagramWidth-2) + "╝\n")
}

// renderSummaryStatistics renders the summary statistics section
func (g *ArchDiagram) renderSummaryStatistics(buffer *bytes.Buffer,
	serviceNodesMap map[ServiceType][]string) {

	// Get node hosts mapping for quick lookup
	nodeHosts := make(map[string]string)
	for _, node := range g.cfg.Nodes {
		nodeHosts[node.Name] = node.Host
	}

	// Calculate actual node counts for all services
	serviceNodeCounts := make(map[ServiceType]int)

	// Calculate node counts for each service
	for svcType, nodeList := range serviceNodesMap {
		// Use a map to track unique IPs for this service
		uniqueIPs := make(map[string]struct{})

		for _, nodeName := range nodeList {
			// Check if this is a node group
			isNodeGroup := false
			for _, nodeGroup := range g.cfg.NodeGroups {
				groupPattern := fmt.Sprintf("%s[%s-%s]", nodeGroup.Name, nodeGroup.IPBegin, nodeGroup.IPEnd)
				if nodeName == groupPattern {
					isNodeGroup = true
					// Add all IPs in this range
					ipList, err := utils.GenerateIPRange(nodeGroup.IPBegin, nodeGroup.IPEnd)
					if err == nil {
						for _, ip := range ipList {
							uniqueIPs[ip] = struct{}{}
						}
					}
					break
				}
			}

			// If not a node group, add the node's IP if available
			if !isNodeGroup {
				if host, ok := nodeHosts[nodeName]; ok {
					uniqueIPs[host] = struct{}{}
				} else if nodeName == "default-client" || nodeName == "default-storage" {
					// Special case for default nodes
					uniqueIPs[nodeName] = struct{}{}
				}
			}
		}

		serviceNodeCounts[svcType] = len(uniqueIPs)
	}

	// Get actual total node count
	totalNodeCount := g.getTotalActualNodeCount()

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

// getNodesForService gets nodes for a specific service type
func (g *ArchDiagram) getNodesForService(nodes []string, nodeGroups []string) []string {
	var serviceNodes []string

	for _, nodeName := range nodes {
		for _, node := range g.cfg.Nodes {
			if node.Name == nodeName {
				serviceNodes = append(serviceNodes, node.Name)
				break
			}
		}
	}

	for _, groupName := range nodeGroups {
		for _, nodeGroup := range g.cfg.NodeGroups {
			if nodeGroup.Name == groupName {
				ipList := g.expandNodeGroup(&nodeGroup)
				serviceNodes = append(serviceNodes, ipList...)
			}
		}
	}

	return serviceNodes
}

// getServiceNodes returns nodes for a specific service type
func (g *ArchDiagram) getServiceNodes(serviceType ServiceType) []string {
	var nodes []string
	var nodeGroups []string

	// Get node and nodeGroup configuration based on service type
	switch serviceType {
	case ServiceMgmtd:
		nodes = g.cfg.Services.Mgmtd.Nodes
		nodeGroups = g.cfg.Services.Mgmtd.NodeGroups
	case ServiceMonitor:
		nodes = g.cfg.Services.Monitor.Nodes
		nodeGroups = g.cfg.Services.Monitor.NodeGroups
	case ServiceStorage:
		nodes = g.cfg.Services.Storage.Nodes
		nodeGroups = g.cfg.Services.Storage.NodeGroups
	case ServiceFdb:
		nodes = g.cfg.Services.Fdb.Nodes
		nodeGroups = g.cfg.Services.Fdb.NodeGroups
	case ServiceClickhouse:
		nodes = g.cfg.Services.Clickhouse.Nodes
		nodeGroups = g.cfg.Services.Clickhouse.NodeGroups
	case ServiceMeta:
		nodes = g.cfg.Services.Meta.Nodes
		nodeGroups = g.cfg.Services.Meta.NodeGroups
	case ServiceClient:
		nodes = g.cfg.Services.Client.Nodes
		nodeGroups = g.cfg.Services.Client.NodeGroups
	default:
		return []string{}
	}

	// Get nodes from configuration
	serviceNodes := g.getNodesForService(nodes, nodeGroups)

	// Special case for client nodes
	if serviceType == ServiceClient && len(serviceNodes) == 0 {
		return []string{"default-client"}
	}

	return serviceNodes
}

// getClientNodes returns all client nodes
func (g *ArchDiagram) getClientNodes() []string {
	return g.getServiceNodes(ServiceClient)
}

// getStorageNodes returns all storage nodes
func (g *ArchDiagram) getStorageNodes() []string {
	// Get all possible storage nodes from config, preserving original order
	var allNodes []string
	nodeMap := make(map[string]struct{})

	// Collect all node information according to the order in config file
	for _, node := range g.cfg.Nodes {
		if _, exists := nodeMap[node.Name]; !exists {
			nodeMap[node.Name] = struct{}{}
			allNodes = append(allNodes, node.Name)
		}
	}

	// Check if each node provides any storage-related services
	seenNodes := make(map[string]bool)
	var displayNodes []string

	// Add nodes in config order, maintaining the original sequence
	for _, nodeName := range allNodes {
		// Check if this node runs any storage-related service
		for _, cfg := range serviceConfigs {
			var serviceNodes []string
			if cfg.Type == ServiceMeta {
				serviceNodes = g.getMetaNodes()
			} else {
				serviceNodes = g.getServiceNodes(cfg.Type)
			}

			if g.isNodeInList(nodeName, serviceNodes) {
				if !seenNodes[nodeName] {
					seenNodes[nodeName] = true
					displayNodes = append(displayNodes, nodeName)
				}
				break
			}
		}
	}

	// Return default value if no storage nodes found
	if len(displayNodes) == 0 {
		return []string{"default-storage"}
	}

	return displayNodes
}

// expandNodeGroup expands a node group into individual IP addresses
func (g *ArchDiagram) expandNodeGroup(nodeGroup *config.NodeGroup) []string {
	// For display purposes, we use a single string to represent the entire group
	nodeName := fmt.Sprintf("%s[%s-%s]", nodeGroup.Name, nodeGroup.IPBegin, nodeGroup.IPEnd)
	return []string{nodeName}
}

// getTotalActualNodeCount calculates the actual total number of physical nodes
func (g *ArchDiagram) getTotalActualNodeCount() int {
	// Create a set to track unique nodes by IP address
	uniqueIPs := make(map[string]struct{})

	// Add regular nodes
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

// isNodeInList checks if a node is in the list
func (g *ArchDiagram) isNodeInList(nodeName string, nodeList []string) bool {
	// For short lists, linear search is fastest
	if len(nodeList) < 10 {
		for _, n := range nodeList {
			if n == nodeName {
				return true
			}
		}
		return false
	}

	// For longer lists, use map for faster lookup
	nodeSet := make(map[string]bool, len(nodeList))
	for _, n := range nodeList {
		nodeSet[n] = true
	}
	return nodeSet[nodeName]
}

// getMetaNodes gets nodes running meta service
func (g *ArchDiagram) getMetaNodes() []string {
	metaNodes := g.getServiceNodes(ServiceMeta)

	if len(metaNodes) == 0 {
		// If no meta nodes configured, use storage and mgmtd nodes
		metaNodes = g.getServiceNodes(ServiceStorage)
		mgmtdNodes := g.getServiceNodes(ServiceMgmtd)

		for _, nodeName := range mgmtdNodes {
			if !g.isNodeInList(nodeName, metaNodes) {
				metaNodes = append(metaNodes, nodeName)
			}
		}
	}

	return metaNodes
}

// getNetworkSpeed gets the network speed
func (g *ArchDiagram) getNetworkSpeed() string {
	// Try to detect speed from the system
	if speed := g.getIBNetworkSpeed(); speed != "" {
		return speed
	}

	if speed := g.getEthernetSpeed(); speed != "" {
		return speed
	}

	// Use default values based on network type
	return g.getDefaultNetworkSpeed()
}

// getDefaultNetworkSpeed returns the default network speed based on the network type
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

// getIBNetworkSpeed gets InfiniBand network speed using a safer approach
func (g *ArchDiagram) getIBNetworkSpeed() string {
	cmdIB := exec.Command("ibstatus")
	output, err := cmdIB.CombinedOutput() // Use CombinedOutput to capture both stdout and stderr
	if err != nil {
		// Just silently fail and return empty, the caller will use defaults
		return ""
	}

	speedPattern := regexp.MustCompile(`rate:\s+(\d+)\s+Gb/sec`)
	matches := speedPattern.FindStringSubmatch(string(output))
	if len(matches) > 1 {
		return matches[1] + " Gb/sec"
	}

	return ""
}

// getEthernetSpeed gets Ethernet network speed
func (g *ArchDiagram) getEthernetSpeed() string {
	interfaceName, err := g.getDefaultInterface()
	if err != nil || interfaceName == "" {
		return ""
	}

	return g.getInterfaceSpeed(interfaceName)
}

// getDefaultInterface returns the name of the default network interface
func (g *ArchDiagram) getDefaultInterface() (string, error) {
	cmdIp := exec.Command("sh", "-c", "ip route | grep default | awk '{print $5}'")
	interfaceOutput, err := cmdIp.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(interfaceOutput)), nil
}

// getInterfaceSpeed returns the speed of the given network interface with improved error handling
func (g *ArchDiagram) getInterfaceSpeed(interfaceName string) string {
	if interfaceName == "" {
		return ""
	}

	cmdEthtool := exec.Command("ethtool", interfaceName)
	ethtoolOutput, err := cmdEthtool.CombinedOutput() // Capture both stdout and stderr
	if err != nil {
		return ""
	}

	speedPattern := regexp.MustCompile(`Speed:\s+(\d+)\s*([GMK]b/?s)`)
	matches := speedPattern.FindStringSubmatch(string(ethtoolOutput))
	if len(matches) > 2 {
		return matches[1] + " " + matches[2]
	}

	return ""
}

// getColorCode returns the appropriate color code based on whether colors are enabled
func (g *ArchDiagram) getColorCode(colorCode string) string {
	if !g.colorEnabled {
		return ""
	}
	return colorCode
}
