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
	if g.colorEnabled {
		return colorReset
	}
	return ""
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
			// Calculate the length of the content in brackets (including brackets)
			labelLength := len(serviceName) + 2 // +2 for the brackets []

			// Fix the total width of the cell content to 16 (aligned with node names)
			// Minus 2 leading spaces, total width is 14
			totalCellWidth := 14

			// Calculate the number of spaces needed
			spacesNeeded := totalCellWidth - labelLength
			if spacesNeeded < 0 {
				spacesNeeded = 0
			}

			// Build the service label string
			serviceLabel := "[" + serviceName + "]"

			buffer.WriteString("|  " + g.getColorCode(color) +
				serviceLabel + g.getColorReset() +
				strings.Repeat(" ", spacesNeeded) + "| ")
		} else {
			buffer.WriteString("|                | ")
		}
	}
	buffer.WriteString("\n")
}

// renderStorageFunc renders all services for storage nodes
func (g *ArchitectureDiagramGenerator) renderStorageFunc(buffer *bytes.Buffer, _ string, startIndex int) {
	storageNodes := g.getStorageNodes()
	realStorageNodes := g.getRealStorageNodes()
	fdbNodes := g.getFdbNodes()
	metaNodes := g.getMetaNodes()
	mgmtdNodes := g.getMgmtdNodes()
	monitorNodes := g.getMonitorNodes()
	clickhouseNodes := g.getClickhouseNodes()

	storageCount := len(storageNodes)
	rowSize := 8
	endIndex := startIndex + rowSize
	if endIndex > storageCount {
		endIndex = storageCount
	}

	// Render all service rows with proper alignment
	g.renderServiceRow(buffer, storageNodes, realStorageNodes, startIndex, endIndex, "storage", colorYellow)
	g.renderServiceRow(buffer, storageNodes, fdbNodes, startIndex, endIndex, "foundationdb", colorBlue)
	g.renderServiceRow(buffer, storageNodes, metaNodes, startIndex, endIndex, "meta", colorPink)
	g.renderServiceRow(buffer, storageNodes, mgmtdNodes, startIndex, endIndex, "mgmtd", colorPurple)
	g.renderServiceRow(buffer, storageNodes, monitorNodes, startIndex, endIndex, "monitor", colorPurple)
	g.renderServiceRow(buffer, storageNodes, clickhouseNodes, startIndex, endIndex, "clickhouse", colorRed)
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
	clientNodes := g.getClientNodes()
	clientCount := len(clientNodes)

	rowSize := 8
	endIndex := startIndex + rowSize
	if endIndex > clientCount {
		endIndex = clientCount
	}

	// Use appropriate service rendering
	g.renderServiceRow(buffer, clientNodes, clientNodes, startIndex, endIndex, "hf3fs_fuse", colorGreen)
}

// GenerateBasicASCII generates a basic ASCII art representation of the cluster architecture
func (g *ArchitectureDiagramGenerator) GenerateBasicASCII() (string, error) {
	var buffer bytes.Buffer

	clientNodes := g.getClientNodes()
	storageNodes := g.getStorageNodes()

	mgmtdNodes := g.getMgmtdNodes()
	monitorNodes := g.getMonitorNodes()
	realStorageNodes := g.getRealStorageNodes()
	fdbNodes := g.getFdbNodes()
	clickhouseNodes := g.getClickhouseNodes()
	metaNodes := g.getMetaNodes()

	networkSpeed := g.getNetworkSpeed()

	buffer.WriteString(fmt.Sprintf("Cluster: %s\n", g.cfg.Name))
	buffer.WriteString(strings.Repeat("=", 70))
	buffer.WriteString("\n\n")

	clientCount := len(clientNodes)
	storageCount := len(storageNodes)

	buffer.WriteString(g.getColorCode(colorCyan) + "CLIENT NODES:" + g.getColorReset() + "\n")
	buffer.WriteString(strings.Repeat("-", 70))
	buffer.WriteString("\n")

	// 使用专门的客户端渲染函数
	g.renderNodeRow(&buffer, clientNodes, 8, g.renderClientFunc)

	buffer.WriteString("\n")

	arrowCount := clientCount
	if arrowCount <= 0 {
		arrowCount = 1
	} else if arrowCount > 15 {
		arrowCount = 15
	}
	buffer.WriteString(strings.Repeat("  ↓ ", arrowCount))
	buffer.WriteString("\n")

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

	arrowCount = storageCount
	if arrowCount <= 0 {
		arrowCount = 1
	} else if arrowCount > 15 {
		arrowCount = 15
	}
	buffer.WriteString(strings.Repeat("  ↓ ", arrowCount))
	buffer.WriteString("\n\n")

	buffer.WriteString(g.getColorCode(colorCyan) + "STORAGE NODES:" + g.getColorReset() + "\n")
	buffer.WriteString(strings.Repeat("-", 70))
	buffer.WriteString("\n")

	// Render storage nodes using the storage function
	g.renderNodeRow(&buffer, storageNodes, 8, g.renderStorageFunc)

	buffer.WriteString("\n" + g.getColorCode(colorCyan) + "CLUSTER SUMMARY:" + g.getColorReset() + "\n")
	buffer.WriteString(strings.Repeat("-", 70) + "\n")

	// Calculate unique nodes count
	var uniqueNodes []string
	for _, node := range clientNodes {
		if !g.isNodeInList(node, uniqueNodes) {
			uniqueNodes = append(uniqueNodes, node)
		}
	}

	for _, node := range storageNodes {
		if !g.isNodeInList(node, uniqueNodes) {
			uniqueNodes = append(uniqueNodes, node)
		}
	}

	// Render summary statistics - first row
	firstRowStats := []struct {
		name  string
		count int
		color string
		width int
	}{
		{"Client Nodes", len(clientNodes), colorGreen, 13},
		{"Storage Nodes", len(realStorageNodes), colorYellow, 14},
		{"FoundationDB", len(fdbNodes), colorBlue, 12},
		{"Meta Service", len(metaNodes), colorPink, 12},
	}
	g.renderSummaryRow(&buffer, firstRowStats)

	// Render summary statistics - second row
	secondRowStats := []struct {
		name  string
		count int
		color string
		width int
	}{
		{"Mgmtd Service", len(mgmtdNodes), colorPurple, 13},
		{"Monitor Svc", len(monitorNodes), colorPurple, 14},
		{"Clickhouse", len(clickhouseNodes), colorRed, 12},
		{"Total Nodes", len(uniqueNodes), colorCyan, 12},
	}
	g.renderSummaryRow(&buffer, secondRowStats)

	return buffer.String(), nil
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

// getClientNodes returns all client nodes
func (g *ArchitectureDiagramGenerator) getClientNodes() []string {
	clientNodeNames := g.getNodesForService(g.cfg.Services.Client.Nodes, g.cfg.Services.Client.NodeGroups)

	if len(clientNodeNames) == 0 {
		return []string{"default-client"}
	}

	return clientNodeNames
}

// getStorageNodes returns all storage nodes
func (g *ArchitectureDiagramGenerator) getStorageNodes() []string {
	mgmtdNodes := g.getMgmtdNodes()
	monitorNodes := g.getMonitorNodes()
	realStorageNodes := g.getRealStorageNodes()
	fdbNodes := g.getFdbNodes()
	metaNodes := g.getMetaNodes()
	clickhouseNodes := g.getClickhouseNodes()

	var displayNodes []string

	for _, nodeName := range mgmtdNodes {
		if !g.isNodeInList(nodeName, displayNodes) {
			displayNodes = append(displayNodes, nodeName)
		}
	}

	for _, nodeName := range monitorNodes {
		if !g.isNodeInList(nodeName, displayNodes) {
			displayNodes = append(displayNodes, nodeName)
		}
	}

	for _, nodeName := range fdbNodes {
		if !g.isNodeInList(nodeName, displayNodes) {
			displayNodes = append(displayNodes, nodeName)
		}
	}

	for _, nodeName := range metaNodes {
		if !g.isNodeInList(nodeName, displayNodes) {
			displayNodes = append(displayNodes, nodeName)
		}
	}

	for _, nodeName := range clickhouseNodes {
		if !g.isNodeInList(nodeName, displayNodes) {
			displayNodes = append(displayNodes, nodeName)
		}
	}

	for _, nodeName := range realStorageNodes {
		if !g.isNodeInList(nodeName, displayNodes) {
			displayNodes = append(displayNodes, nodeName)
		}
	}

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

// getMgmtdNodes gets all mgmtd nodes
func (g *ArchitectureDiagramGenerator) getMgmtdNodes() []string {
	return g.getNodesForService(g.cfg.Services.Mgmtd.Nodes, g.cfg.Services.Mgmtd.NodeGroups)
}

// getMonitorNodes gets all monitor nodes
func (g *ArchitectureDiagramGenerator) getMonitorNodes() []string {
	return g.getNodesForService(g.cfg.Services.Monitor.Nodes, g.cfg.Services.Monitor.NodeGroups)
}

// getRealStorageNodes gets the actual storage nodes, excluding nodes that only run management services
func (g *ArchitectureDiagramGenerator) getRealStorageNodes() []string {
	return g.getNodesForService(g.cfg.Services.Storage.Nodes, g.cfg.Services.Storage.NodeGroups)
}

// getFdbNodes gets nodes running foundationdb service
func (g *ArchitectureDiagramGenerator) getFdbNodes() []string {
	return g.getNodesForService(g.cfg.Services.Fdb.Nodes, g.cfg.Services.Fdb.NodeGroups)
}

// getClickhouseNodes gets nodes running clickhouse service
func (g *ArchitectureDiagramGenerator) getClickhouseNodes() []string {
	return g.getNodesForService(g.cfg.Services.Clickhouse.Nodes, g.cfg.Services.Clickhouse.NodeGroups)
}

// getMetaNodes gets nodes running meta service
func (g *ArchitectureDiagramGenerator) getMetaNodes() []string {
	metaNodes := g.getNodesForService(g.cfg.Services.Meta.Nodes, g.cfg.Services.Meta.NodeGroups)

	if len(metaNodes) == 0 {
		metaNodes = g.getRealStorageNodes()
		mgmtdNodes := g.getMgmtdNodes()
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
	var speed string

	speedIB := g.getIBNetworkSpeed()
	if speedIB != "" {
		speed = speedIB
		return speed
	}

	speedEth := g.getEthernetSpeed()
	if speedEth != "" {
		speed = speedEth
		return speed
	}

	if g.cfg.NetworkType == config.NetworkTypeIB {
		return "50 Gb/sec"
	}
	if g.cfg.NetworkType == config.NetworkTypeRDMA {
		return "100 Gb/sec"
	}
	return "10 Gb/sec"
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
	if g.colorEnabled {
		return colorCode
	}
	return ""
}
