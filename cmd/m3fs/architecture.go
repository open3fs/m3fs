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

// ArchitectureDiagramGenerator generates architecture diagrams for m3fs clusters
type ArchitectureDiagramGenerator struct {
	cfg          *config.Config
	colorEnabled bool
}

// NewArchitectureDiagramGenerator creates a new ArchitectureDiagramGenerator
func NewArchitectureDiagramGenerator(cfg *config.Config) *ArchitectureDiagramGenerator {
	return &ArchitectureDiagramGenerator{
		cfg:          cfg,
		colorEnabled: true,
	}
}

// SetColorEnabled enables or disables colored output in the diagram
func (g *ArchitectureDiagramGenerator) SetColorEnabled(enabled bool) {
	g.colorEnabled = enabled
}

// Generate generates an architecture diagram
func (g *ArchitectureDiagramGenerator) Generate() (string, error) {
	return g.GenerateBasicASCII()
}

// getColorReset returns the color reset code if colors are enabled
func (g *ArchitectureDiagramGenerator) getColorReset() string {
	return g.getColorCode(colorReset)
}

// renderNodeRow renders a row of nodes with the given style
func (g *ArchitectureDiagramGenerator) renderNodeRow(buffer *bytes.Buffer, nodes []string, rowSize int,
	renderFunc func(buffer *bytes.Buffer, nodeName string, index int)) {

	nodeCount := len(nodes)
	for i := 0; i < nodeCount; i += rowSize {
		end := i + rowSize
		if end > nodeCount {
			end = nodeCount
		}

		for j := i; j < end; j++ {
			buffer.WriteString("+----------------+ ")
		}
		buffer.WriteString("\n")

		for j := i; j < end; j++ {
			nodeName := nodes[j]
			if len(nodeName) > 16 {
				nodeName = nodeName[:13] + "..."
			}
			buffer.WriteString("|" + g.getColorCode(colorCyan) + fmt.Sprintf("%-16s", nodeName) + g.getColorReset() + "| ")
		}
		buffer.WriteString("\n")

		renderFunc(buffer, "", i)

		for j := i; j < end; j++ {
			buffer.WriteString("+----------------+ ")
		}
		buffer.WriteString("\n")
	}
}

// renderServiceRow renders a row showing a specific service on the nodes
func (g *ArchitectureDiagramGenerator) renderServiceRow(buffer *bytes.Buffer,
	nodes []string, serviceNodes []string, startIndex int, endIndex int,
	serviceName string, color string) {

	for j := startIndex; j < endIndex; j++ {
		nodeName := nodes[j]
		if g.isNodeInList(nodeName, serviceNodes) {
			serviceLabel := "[" + serviceName + "]"
			totalCellWidth := 14
			paddingNeeded := totalCellWidth - len(serviceLabel)
			if paddingNeeded < 0 {
				paddingNeeded = 0
			}

			buffer.WriteString("|  " + g.getColorCode(color) +
				serviceLabel + g.getColorReset() +
				strings.Repeat(" ", paddingNeeded) + "| ")
		} else {
			buffer.WriteString("|                | ")
		}
	}
	buffer.WriteString("\n")
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

	defaultRowSize = 8
)

// renderStorageFunc renders all services for storage nodes
func (g *ArchitectureDiagramGenerator) renderStorageFunc(buffer *bytes.Buffer, _ string, startIndex int) {
	storageNodes := g.getStorageNodes()
	
	storageCount := len(storageNodes)
	endIndex := startIndex + defaultRowSize
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

// renderSummaryRow renders a row of statistics
func (g *ArchitectureDiagramGenerator) renderSummaryRow(buffer *bytes.Buffer, stats []struct {
	name  string
	count int
	color string
	width int
}) {
	for _, stat := range stats {
		buffer.WriteString(fmt.Sprintf("%s%-"+fmt.Sprintf("%d", stat.width)+"s%s %-2d  ",
			g.getColorCode(stat.color), stat.name+":", g.getColorReset(), stat.count))
	}
	buffer.WriteString("\n")
}

// renderClientFunc renders client services
func (g *ArchitectureDiagramGenerator) renderClientFunc(buffer *bytes.Buffer, _ string, startIndex int) {
	clientNodes := g.getServiceNodes(ServiceClient)
	clientCount := len(clientNodes)

	endIndex := startIndex + defaultRowSize
	if endIndex > clientCount {
		endIndex = clientCount
	}

	g.renderServiceRow(buffer, clientNodes, clientNodes, startIndex, endIndex, "hf3fs_fuse", colorGreen)
}

// GenerateBasicASCII generates a basic ASCII art representation of the cluster architecture
func (g *ArchitectureDiagramGenerator) GenerateBasicASCII() (string, error) {
	var buffer bytes.Buffer

	clientNodes := g.getServiceNodes(ServiceClient)
	storageNodes := g.getStorageNodes()
	
	// Create service nodes map for use in statistics
	serviceNodesMap := make(map[ServiceType][]string)
	
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

	networkSpeed := g.getNetworkSpeed()

	// Render cluster header
	buffer.WriteString(fmt.Sprintf("Cluster: %s\n", g.cfg.Name))
	buffer.WriteString(strings.Repeat("=", 70))
	buffer.WriteString("\n\n")

	// Client nodes section
	buffer.WriteString(g.getColorCode(colorCyan) + "CLIENT NODES:" + g.getColorReset() + "\n")
	buffer.WriteString(strings.Repeat("-", 70))
	buffer.WriteString("\n")

	g.renderNodeRow(&buffer, clientNodes, defaultRowSize, g.renderClientFunc)
	buffer.WriteString("\n")

	// Calculate appropriate arrow count
	arrowCount := g.calculateArrowCount(len(clientNodes))
	buffer.WriteString(strings.Repeat("  ↓ ", arrowCount))
	buffer.WriteString("\n")

	// Network section
	g.renderNetworkSection(&buffer, networkSpeed)

	// Calculate storage arrow count
	arrowCount = g.calculateArrowCount(len(storageNodes))
	buffer.WriteString(strings.Repeat("  ↓ ", arrowCount))
	buffer.WriteString("\n\n")

	// Storage nodes section
	buffer.WriteString(g.getColorCode(colorCyan) + "STORAGE NODES:" + g.getColorReset() + "\n")
	buffer.WriteString(strings.Repeat("-", 70))
	buffer.WriteString("\n")

	g.renderNodeRow(&buffer, storageNodes, defaultRowSize, g.renderStorageFunc)

	// Cluster summary section
	buffer.WriteString("\n" + g.getColorCode(colorCyan) + "CLUSTER SUMMARY:" + g.getColorReset() + "\n")
	buffer.WriteString(strings.Repeat("-", 70) + "\n")

	// Count unique nodes across client and storage
	uniqueNodes := g.countUniqueNodes(clientNodes, storageNodes)

	// Render summary statistics
	g.renderSummaryStatistics(&buffer, serviceNodesMap, uniqueNodes)

	return buffer.String(), nil
}

// calculateArrowCount calculates appropriate arrow count for display
func (g *ArchitectureDiagramGenerator) calculateArrowCount(nodeCount int) int {
	if nodeCount <= 0 {
		return 1
	} else if nodeCount > 15 {
		return 15
	}
	return nodeCount
}

// renderNetworkSection renders the network section of the diagram
func (g *ArchitectureDiagramGenerator) renderNetworkSection(buffer *bytes.Buffer, networkSpeed string) {
	networkText := fmt.Sprintf(" %s Network (%s) ", g.cfg.NetworkType, networkSpeed)
	totalWidth := 70

	buffer.WriteString("╔" + strings.Repeat("═", totalWidth-2) + "╗\n")
	rightPadding := totalWidth - 2 - len(networkText)
	buffer.WriteString("║" +
		g.getColorCode(colorBlue) +
		networkText +
		g.getColorReset() +
		strings.Repeat(" ", rightPadding) +
		"║\n")
	buffer.WriteString("╚" + strings.Repeat("═", totalWidth-2) + "╝\n")
}

// countUniqueNodes counts unique nodes across node lists
func (g *ArchitectureDiagramGenerator) countUniqueNodes(nodeLists ...[]string) []string {
	nodeMap := make(map[string]bool)
	
	for _, list := range nodeLists {
		for _, node := range list {
			nodeMap[node] = true
		}
	}
	
	uniqueNodes := make([]string, 0, len(nodeMap))
	for node := range nodeMap {
		uniqueNodes = append(uniqueNodes, node)
	}
	
	return uniqueNodes
}

// renderSummaryStatistics renders the summary statistics section
func (g *ArchitectureDiagramGenerator) renderSummaryStatistics(buffer *bytes.Buffer, 
	serviceNodesMap map[ServiceType][]string, uniqueNodes []string) {
	
	firstRowStats := []struct {
		name  string
		count int
		color string
		width int
	}{
		{"Client Nodes", len(serviceNodesMap[ServiceClient]), colorGreen, 13},
		{"Storage Nodes", len(serviceNodesMap[ServiceStorage]), colorYellow, 14},
		{"FoundationDB", len(serviceNodesMap[ServiceFdb]), colorBlue, 12},
		{"Meta Service", len(serviceNodesMap[ServiceMeta]), colorPink, 12},
	}
	g.renderSummaryRow(buffer, firstRowStats)

	secondRowStats := []struct {
		name  string
		count int
		color string
		width int
	}{
		{"Mgmtd Service", len(serviceNodesMap[ServiceMgmtd]), colorPurple, 13},
		{"Monitor Svc", len(serviceNodesMap[ServiceMonitor]), colorPurple, 14},
		{"Clickhouse", len(serviceNodesMap[ServiceClickhouse]), colorRed, 12},
		{"Total Nodes", len(uniqueNodes), colorCyan, 12},
	}
	g.renderSummaryRow(buffer, secondRowStats)
}

// getNodesForService gets nodes for a specific service type
func (g *ArchitectureDiagramGenerator) getNodesForService(nodes []string, nodeGroups []string) []string {
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
func (g *ArchitectureDiagramGenerator) getServiceNodes(serviceType ServiceType) []string {
	switch serviceType {
	case ServiceMgmtd:
		return g.getNodesForService(g.cfg.Services.Mgmtd.Nodes, g.cfg.Services.Mgmtd.NodeGroups)
	case ServiceMonitor:
		return g.getNodesForService(g.cfg.Services.Monitor.Nodes, g.cfg.Services.Monitor.NodeGroups)
	case ServiceStorage:
		return g.getNodesForService(g.cfg.Services.Storage.Nodes, g.cfg.Services.Storage.NodeGroups)
	case ServiceFdb:
		return g.getNodesForService(g.cfg.Services.Fdb.Nodes, g.cfg.Services.Fdb.NodeGroups)
	case ServiceClickhouse:
		return g.getNodesForService(g.cfg.Services.Clickhouse.Nodes, g.cfg.Services.Clickhouse.NodeGroups)
	case ServiceMeta:
		return g.getNodesForService(g.cfg.Services.Meta.Nodes, g.cfg.Services.Meta.NodeGroups)
	case ServiceClient:
		clientNodeNames := g.getNodesForService(g.cfg.Services.Client.Nodes, g.cfg.Services.Client.NodeGroups)
		if len(clientNodeNames) == 0 {
			return []string{"default-client"}
		}
		return clientNodeNames
	default:
		return []string{}
	}
}

// getClientNodes returns all client nodes
func (g *ArchitectureDiagramGenerator) getClientNodes() []string {
	return g.getServiceNodes(ServiceClient)
}

// getStorageNodes returns all storage nodes
func (g *ArchitectureDiagramGenerator) getStorageNodes() []string {
	// 首先获取配置中所有可能的存储节点
	var allNodes []string
	
	// 收集所有节点信息
	for _, node := range g.cfg.Nodes {
		allNodes = append(allNodes, node.Name)
	}
	
	// 检查每个节点是否提供任何存储相关服务
	seenNodes := make(map[string]bool)
	var displayNodes []string
	
	// 按照配置中的节点顺序添加，确保保持原始顺序
	for _, nodeName := range allNodes {
		// 检查此节点是否运行了任何存储相关服务
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
	
	// 如果没有找到存储节点，返回默认值
	if len(displayNodes) == 0 {
		return []string{"default-storage"}
	}
	
	return displayNodes
}

// expandNodeGroup expands a node group into individual IP addresses
func (g *ArchitectureDiagramGenerator) expandNodeGroup(nodeGroup *config.NodeGroup) []string {
	nodeName := fmt.Sprintf("%s[%s-%s]", nodeGroup.Name, nodeGroup.IPBegin, nodeGroup.IPEnd)
	return []string{nodeName}
}

// isNodeInList checks if a node is in the list
func (g *ArchitectureDiagramGenerator) isNodeInList(nodeName string, nodeList []string) bool {
	for _, n := range nodeList {
		if n == nodeName {
			return true
		}
	}
	return false
}

// getMetaNodes gets nodes running meta service
func (g *ArchitectureDiagramGenerator) getMetaNodes() []string {
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
func (g *ArchitectureDiagramGenerator) getNetworkSpeed() string {
	// Try to detect speed from the system
	if speed := g.getIBNetworkSpeed(); speed != "" {
		return speed
	}
	
	if speed := g.getEthernetSpeed(); speed != "" {
		return speed
	}
	
	// Use default values based on network type
	switch g.cfg.NetworkType {
	case config.NetworkTypeIB:
		return "50 Gb/sec"
	case config.NetworkTypeRDMA:
		return "100 Gb/sec"
	default:
		return "10 Gb/sec"
	}
}

// getIBNetworkSpeed gets InfiniBand network speed
func (g *ArchitectureDiagramGenerator) getIBNetworkSpeed() string {
	cmd := exec.Command("ibstatus")
	output, err := cmd.Output()
	if err != nil {
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
func (g *ArchitectureDiagramGenerator) getEthernetSpeed() string {
	cmdIp := exec.Command("sh", "-c", "ip route | grep default | awk '{print $5}'")
	interfaceOutput, err := cmdIp.Output()
	if err != nil {
		return ""
	}

	interfaceName := strings.TrimSpace(string(interfaceOutput))
	if interfaceName == "" {
		return ""
	}

	cmdEthtool := exec.Command("ethtool", interfaceName)
	ethtoolOutput, err := cmdEthtool.Output()
	if err != nil {
		return ""
	}

	speedPattern := regexp.MustCompile(`Speed:\s+(\d+)([GMK]b/s)`)
	matches := speedPattern.FindStringSubmatch(string(ethtoolOutput))
	if len(matches) > 2 {
		return matches[1] + " " + matches[2]
	}

	return ""
}

// getColorCode returns the appropriate color code based on whether colors are enabled
func (g *ArchitectureDiagramGenerator) getColorCode(colorCode string) string {
	if !g.colorEnabled || colorCode == "" {
		return ""
	}
	if colorCode == colorReset {
		return colorReset
	}
	return colorCode
}
