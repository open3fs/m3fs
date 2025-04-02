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
		colorEnabled: true, // Default to colored output
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

	rowSize := 8
	for i := 0; i < clientCount; i += rowSize {
		end := i + rowSize
		if end > clientCount {
			end = clientCount
		}

		for j := i; j < end; j++ {
			buffer.WriteString("+----------------+ ")
		}
		buffer.WriteString("\n")

		for j := i; j < end; j++ {
			nodeName := clientNodes[j]
			if len(nodeName) > 16 {
				nodeName = nodeName[:13] + "..."
			}
			buffer.WriteString("|" + g.getColorCode(colorCyan) + fmt.Sprintf("%-16s", nodeName) + g.getColorReset() + "| ")
		}
		buffer.WriteString("\n")

		for j := i; j < end; j++ {
			buffer.WriteString("|  " + g.getColorCode(colorGreen) + "[hf3fs_fuse]" + g.getColorReset() + "  | ")
		}
		buffer.WriteString("\n")

		for j := i; j < end; j++ {
			buffer.WriteString("+----------------+ ")
		}
		buffer.WriteString("\n\n")
	}

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

	for i := 0; i < storageCount; i += rowSize {
		end := i + rowSize
		if end > storageCount {
			end = storageCount
		}

		for j := i; j < end; j++ {
			buffer.WriteString("+----------------+ ")
		}
		buffer.WriteString("\n")

		for j := i; j < end; j++ {
			nodeName := storageNodes[j]
			if len(nodeName) > 16 {
				nodeName = nodeName[:13] + "..."
			}
			buffer.WriteString("|" + g.getColorCode(colorCyan) + fmt.Sprintf("%-16s", nodeName) + g.getColorReset() + "| ")
		}
		buffer.WriteString("\n")

		for j := i; j < end; j++ {
			nodeName := storageNodes[j]
			if g.isNodeInList(nodeName, realStorageNodes) {
				buffer.WriteString("|  " + g.getColorCode(colorYellow) + "[storage]" + g.getColorReset() + "     | ")
			} else {
				buffer.WriteString("|                | ")
			}
		}
		buffer.WriteString("\n")

		for j := i; j < end; j++ {
			nodeName := storageNodes[j]
			if g.isNodeInList(nodeName, fdbNodes) {
				buffer.WriteString("|  " + g.getColorCode(colorBlue) + "[foundationdb]" + g.getColorReset() + "| ")
			} else {
				buffer.WriteString("|                | ")
			}
		}
		buffer.WriteString("\n")

		for j := i; j < end; j++ {
			nodeName := storageNodes[j]
			if g.isNodeInList(nodeName, metaNodes) {
				buffer.WriteString("|  " + g.getColorCode(colorPink) + "[meta]" + g.getColorReset() + "        | ")
			} else {
				buffer.WriteString("|                | ")
			}
		}
		buffer.WriteString("\n")

		for j := i; j < end; j++ {
			nodeName := storageNodes[j]
			if g.isNodeInList(nodeName, mgmtdNodes) {
				buffer.WriteString("|  " + g.getColorCode(colorPurple) + "[mgmtd]" + g.getColorReset() + "       | ")
			} else {
				buffer.WriteString("|                | ")
			}
		}
		buffer.WriteString("\n")

		for j := i; j < end; j++ {
			nodeName := storageNodes[j]
			if g.isNodeInList(nodeName, monitorNodes) {
				buffer.WriteString("|  " + g.getColorCode(colorPurple) + "[monitor]" + g.getColorReset() + "     | ")
			} else {
				buffer.WriteString("|                | ")
			}
		}
		buffer.WriteString("\n")

		for j := i; j < end; j++ {
			nodeName := storageNodes[j]
			if g.isNodeInList(nodeName, clickhouseNodes) {
				buffer.WriteString("|  " + g.getColorCode(colorRed) + "[clickhouse]" + g.getColorReset() + "  | ")
			} else {
				buffer.WriteString("|                | ")
			}
		}
		buffer.WriteString("\n")

		for j := i; j < end; j++ {
			buffer.WriteString("+----------------+ ")
		}
		buffer.WriteString("\n\n")
	}

	return buffer.String(), nil
}

// getClientNodes returns all client nodes
func (g *ArchitectureDiagramGenerator) getClientNodes() []string {
	var clientNodeNames []string

	for _, nodeName := range g.cfg.Services.Client.Nodes {
		for _, node := range g.cfg.Nodes {
			if node.Name == nodeName {
				clientNodeNames = append(clientNodeNames, node.Name)
				break
			}
		}
	}

	for _, groupName := range g.cfg.Services.Client.NodeGroups {
		for _, nodeGroup := range g.cfg.NodeGroups {
			if nodeGroup.Name == groupName {
				ipList := g.expandNodeGroup(&nodeGroup)
				clientNodeNames = append(clientNodeNames, ipList...)
			}
		}
	}

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
	var mgmtdNodes []string

	for _, nodeName := range g.cfg.Services.Mgmtd.Nodes {
		for _, node := range g.cfg.Nodes {
			if node.Name == nodeName {
				mgmtdNodes = append(mgmtdNodes, node.Name)
				break
			}
		}
	}

	for _, groupName := range g.cfg.Services.Mgmtd.NodeGroups {
		for _, nodeGroup := range g.cfg.NodeGroups {
			if nodeGroup.Name == groupName {
				ipList := g.expandNodeGroup(&nodeGroup)
				mgmtdNodes = append(mgmtdNodes, ipList...)
			}
		}
	}

	return mgmtdNodes
}

// getMonitorNodes gets all monitor nodes
func (g *ArchitectureDiagramGenerator) getMonitorNodes() []string {
	var monitorNodes []string

	for _, nodeName := range g.cfg.Services.Monitor.Nodes {
		for _, node := range g.cfg.Nodes {
			if node.Name == nodeName {
				monitorNodes = append(monitorNodes, node.Name)
				break
			}
		}
	}

	for _, groupName := range g.cfg.Services.Monitor.NodeGroups {
		for _, nodeGroup := range g.cfg.NodeGroups {
			if nodeGroup.Name == groupName {
				ipList := g.expandNodeGroup(&nodeGroup)
				monitorNodes = append(monitorNodes, ipList...)
			}
		}
	}

	return monitorNodes
}

// getRealStorageNodes gets the actual storage nodes, excluding nodes that only run management services
func (g *ArchitectureDiagramGenerator) getRealStorageNodes() []string {
	var storageNodeNames []string

	for _, nodeName := range g.cfg.Services.Storage.Nodes {
		for _, node := range g.cfg.Nodes {
			if node.Name == nodeName {
				storageNodeNames = append(storageNodeNames, node.Name)
				break
			}
		}
	}

	for _, groupName := range g.cfg.Services.Storage.NodeGroups {
		for _, nodeGroup := range g.cfg.NodeGroups {
			if nodeGroup.Name == groupName {
				ipList := g.expandNodeGroup(&nodeGroup)
				storageNodeNames = append(storageNodeNames, ipList...)
			}
		}
	}

	return storageNodeNames
}

// getFdbNodes gets nodes running foundationdb service
func (g *ArchitectureDiagramGenerator) getFdbNodes() []string {
	var fdbNodes []string

	for _, nodeName := range g.cfg.Services.Fdb.Nodes {
		for _, node := range g.cfg.Nodes {
			if node.Name == nodeName {
				fdbNodes = append(fdbNodes, node.Name)
				break
			}
		}
	}

	for _, groupName := range g.cfg.Services.Fdb.NodeGroups {
		for _, nodeGroup := range g.cfg.NodeGroups {
			if nodeGroup.Name == groupName {
				ipList := g.expandNodeGroup(&nodeGroup)
				fdbNodes = append(fdbNodes, ipList...)
			}
		}
	}

	return fdbNodes
}

// getClickhouseNodes gets nodes running clickhouse service
func (g *ArchitectureDiagramGenerator) getClickhouseNodes() []string {
	var clickhouseNodes []string

	for _, nodeName := range g.cfg.Services.Clickhouse.Nodes {
		for _, node := range g.cfg.Nodes {
			if node.Name == nodeName {
				clickhouseNodes = append(clickhouseNodes, node.Name)
				break
			}
		}
	}

	for _, groupName := range g.cfg.Services.Clickhouse.NodeGroups {
		for _, nodeGroup := range g.cfg.NodeGroups {
			if nodeGroup.Name == groupName {
				ipList := g.expandNodeGroup(&nodeGroup)
				clickhouseNodes = append(clickhouseNodes, ipList...)
			}
		}
	}

	return clickhouseNodes
}

// getMetaNodes gets nodes running meta service
func (g *ArchitectureDiagramGenerator) getMetaNodes() []string {
	var metaNodes []string

	for _, nodeName := range g.cfg.Services.Meta.Nodes {
		for _, node := range g.cfg.Nodes {
			if node.Name == nodeName {
				metaNodes = append(metaNodes, node.Name)
				break
			}
		}
	}

	for _, groupName := range g.cfg.Services.Meta.NodeGroups {
		for _, nodeGroup := range g.cfg.NodeGroups {
			if nodeGroup.Name == groupName {
				ipList := g.expandNodeGroup(&nodeGroup)
				metaNodes = append(metaNodes, ipList...)
			}
		}
	}

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
