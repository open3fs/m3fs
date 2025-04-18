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

package render

import (
	"fmt"
	"strings"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/utils"
)

// NodeServicesFunc is a function type that returns services for a node
type NodeServicesFunc func(nodeName string) []string

// ServiceCountFunc is a function that returns node counts for each service type
type ServiceCountFunc func() map[config.ServiceType]int

// ArchDiagramRenderer extends DiagramRenderer with architecture diagram specific rendering
type ArchDiagramRenderer struct {
	Base *DiagramRenderer
}

// NewArchDiagramRenderer creates a new ArchDiagramRenderer
func NewArchDiagramRenderer(baseRenderer *DiagramRenderer) *ArchDiagramRenderer {
	return &ArchDiagramRenderer{
		Base: baseRenderer,
	}
}

// CalculateArrowCount calculates the number of arrows to display based on network width
func (r *ArchDiagramRenderer) CalculateArrowCount(nodeRowWidth int) int {
	const arrowWidth = 7 // "   ↓   "

	innerWidth := nodeRowWidth - 2
	arrowCount := innerWidth / arrowWidth

	if arrowCount <= 0 {
		return 1
	} else if arrowCount > 15 {
		return 15
	}
	return arrowCount
}

// RenderArrows renders arrows that align with the network frame
// maxArrows parameter limits the maximum number of arrows to the node count
func (r *ArchDiagramRenderer) RenderArrows(sb *strings.Builder, networkWidth int, maxArrows int) {
	const arrowStr = "   ↓   " // Arrow string with equal spaces on both sides

	arrowCount := r.CalculateArrowCount(networkWidth)

	if maxArrows > 0 && arrowCount > maxArrows {
		arrowCount = maxArrows
	}

	totalArrowWidth := len(arrowStr) * arrowCount
	innerWidth := networkWidth - 2
	leftPadding := (innerWidth - totalArrowWidth) / 2

	rightShift := arrowCount

	if arrowCount <= 4 {
	} else if arrowCount <= 6 {
		rightShift++
	} else {
		rightShift += 2
	}
	leftPadding += rightShift

	if leftPadding < 0 {
		leftPadding = 0
	}

	arrowBuilder := r.Base.GetStringBuilder()
	arrowBuilder.Grow(networkWidth)
	arrowBuilder.WriteString(strings.Repeat(" ", leftPadding))

	for i := 0; i < arrowCount; i++ {
		arrowBuilder.WriteString(arrowStr)
	}

	sb.WriteString(arrowBuilder.String())
	r.Base.PutStringBuilder(arrowBuilder)
}

// RenderNetworkSection renders the network section
func (r *ArchDiagramRenderer) RenderNetworkSection(sb *strings.Builder, networkType string, networkSpeed string) {
	clientNodes := r.Base.lastClientNodesCount

	// Omit "Network" word when only one client to prevent text overflow
	networkText := ""
	if clientNodes == 1 {
		networkText = fmt.Sprintf("%s (%s)", networkType, networkSpeed)
	} else {
		networkText = fmt.Sprintf("%s Network (%s)", networkType, networkSpeed)
	}

	if clientNodes <= 0 {
		clientNodes = 3 // Default minimum width
	}

	actualNodeCount := clientNodes
	if actualNodeCount > r.Base.RowSize {
		actualNodeCount = r.Base.RowSize
	}

	nodeRowWidth := (r.Base.NodeCellWidth + 3) * actualNodeCount
	totalInnerWidth := nodeRowWidth - 2
	leftPadding := (totalInnerWidth - len(networkText)) / 2
	rightPadding := totalInnerWidth - len(networkText) - leftPadding

	if leftPadding < 0 {
		leftPadding = 0
	}
	if rightPadding < 0 {
		rightPadding = 0
	}

	sb.WriteString("╔" + strings.Repeat("═", nodeRowWidth-2) + "╗\n")
	sb.WriteString("║" + strings.Repeat(" ", leftPadding))
	r.Base.RenderWithColor(sb, networkText, ColorBlue)
	sb.WriteString(strings.Repeat(" ", rightPadding) + "║\n")
	sb.WriteString("╚" + strings.Repeat("═", nodeRowWidth-2) + "╝\n")
}

// RenderCustomWidthDivider renders a divider with the specified width
func (r *ArchDiagramRenderer) RenderCustomWidthDivider(sb *strings.Builder, char string, nodeCount int) {
	width := (r.Base.NodeCellWidth + 3) * nodeCount
	sb.WriteString(strings.Repeat(char, width))
	sb.WriteByte('\n')
}

// RenderHeader renders the diagram header using a width based on the first row of nodes
func (r *ArchDiagramRenderer) RenderHeader(sb *strings.Builder, nodeCount int) {
	replicationFactor := r.Base.cfg.Services.Storage.ReplicationFactor
	titleText := fmt.Sprintf("Cluster: %s  replicationFactor: %d", r.Base.cfg.Name, replicationFactor)

	r.Base.RenderLine(sb, titleText, "")
	r.RenderCustomWidthDivider(sb, "=", nodeCount)
	sb.WriteByte('\n')
}

// RenderCustomSectionHeader renders a section header with custom width
func (r *ArchDiagramRenderer) RenderCustomSectionHeader(sb *strings.Builder, title string, nodeCount int) {
	r.Base.RenderWithColor(sb, title, ColorCyan)
	sb.WriteByte('\n')
	r.RenderCustomWidthDivider(sb, "-", nodeCount)
}

// RenderClientSection renders the client nodes section
func (r *ArchDiagramRenderer) RenderClientSection(sb *strings.Builder, clientNodes []string) {
	nodeCount := utils.Min(len(clientNodes), r.Base.RowSize)
	r.RenderCustomSectionHeader(sb, "CLIENT NODES:", nodeCount)

	clientCount := len(clientNodes)
	r.Base.lastClientNodesCount = clientCount

	for i := 0; i < clientCount; i += r.Base.RowSize {
		end := i + r.Base.RowSize
		if end > clientCount {
			end = clientCount
		}
		r.RenderClientNodes(sb, clientNodes[i:end])
	}

	sb.WriteByte('\n')
	nodeRowWidth := (r.Base.NodeCellWidth + 3) * nodeCount
	r.RenderArrows(sb, nodeRowWidth, nodeCount)
	sb.WriteByte('\n')
}

// RenderClientNodes renders client nodes
func (r *ArchDiagramRenderer) RenderClientNodes(sb *strings.Builder, clientNodes []string) {
	if len(clientNodes) == 0 {
		return
	}

	hostMountpoint := ""
	if r.Base.cfg != nil && r.Base.cfg.Services.Client.HostMountpoint != "" {
		hostMountpoint = r.Base.cfg.Services.Client.HostMountpoint
	} else {
		hostMountpoint = "/mnt/3fs"
	}

	mountpointStr := fmt.Sprintf("[%s]", hostMountpoint)

	r.Base.RenderNodesRow(sb, clientNodes, func(node string) []string {
		return []string{
			"[hf3fs_fuse]",
			mountpointStr,
		}
	}, false)
}

// RenderStorageSection renders the storage nodes section
func (r *ArchDiagramRenderer) RenderStorageSection(
	sb *strings.Builder,
	storageNodes []string,
	nodeServicesFunc NodeServicesFunc,
) {
	storageCount := len(storageNodes)
	firstRowCount := storageCount
	if firstRowCount > r.Base.RowSize {
		firstRowCount = r.Base.RowSize
	}

	storageNodeCount := utils.Min(storageCount, r.Base.RowSize)
	nodeRowWidth := (r.Base.NodeCellWidth + 3) * utils.Min(r.Base.lastClientNodesCount, r.Base.RowSize)

	r.RenderArrows(sb, nodeRowWidth, storageNodeCount)

	sb.WriteString("\n\n")
	r.RenderCustomSectionHeader(sb, "STORAGE NODES:", firstRowCount)

	for i := 0; i < storageCount; i += r.Base.RowSize {
		end := i + r.Base.RowSize
		if end > storageCount {
			end = storageCount
		}
		r.RenderStorageNodes(sb, storageNodes[i:end], nodeServicesFunc)
	}
}

// RenderStorageNodes renders storage nodes
func (r *ArchDiagramRenderer) RenderStorageNodes(
	sb *strings.Builder,
	storageNodes []string,
	nodeServicesFunc NodeServicesFunc,
) {
	if len(storageNodes) == 0 {
		return
	}

	r.Base.RenderNodesRow(sb, storageNodes, nodeServicesFunc, true)
}

// RenderSummarySection renders the summary section
func (r *ArchDiagramRenderer) RenderSummarySection(
	sb *strings.Builder,
	serviceCountsFunc ServiceCountFunc,
	totalNodeCount int,
) {
	sb.WriteString("\n")
	r.Base.RenderSectionHeader(sb, "CLUSTER SUMMARY:")
	r.RenderSummaryStatistics(sb, serviceCountsFunc, totalNodeCount)
}

// RenderSummaryStatistics renders the summary statistics
func (r *ArchDiagramRenderer) RenderSummaryStatistics(
	sb *strings.Builder,
	serviceCountsFunc ServiceCountFunc,
	totalNodeCount int,
) {
	serviceNodeCounts := serviceCountsFunc()

	firstRow := []struct {
		name  string
		count int
		color string
	}{
		{"Client Nodes", serviceNodeCounts[config.ServiceClient], ColorGreen},
		{"Storage Nodes", serviceNodeCounts[config.ServiceStorage], ColorYellow},
		{"FoundationDB", serviceNodeCounts[config.ServiceFdb], ColorBlue},
		{"Meta Service", serviceNodeCounts[config.ServiceMeta], ColorPink},
	}

	for _, stat := range firstRow {
		sb.WriteString(fmt.Sprintf("%s%-14s%s %-2d  ",
			r.Base.GetColorCode(stat.color),
			stat.name+":",
			r.Base.GetColorReset(),
			stat.count))
	}
	sb.WriteByte('\n')

	secondRow := []struct {
		name  string
		count int
		color string
	}{
		{"Mgmtd Service", serviceNodeCounts[config.ServiceMgmtd], ColorPurple},
		{"Monitor Svc", serviceNodeCounts[config.ServiceMonitor], ColorPurple},
		{"Clickhouse", serviceNodeCounts[config.ServiceClickhouse], ColorRed},
		{"Total Nodes", totalNodeCount, ColorCyan},
	}

	for _, stat := range secondRow {
		sb.WriteString(fmt.Sprintf("%s%-14s%s %-2d  ",
			r.Base.GetColorCode(stat.color),
			stat.name+":",
			r.Base.GetColorReset(),
			stat.count))
	}
	sb.WriteByte('\n')
}

// SetColorEnabled enables or disables color output
func (r *ArchDiagramRenderer) SetColorEnabled(enabled bool) {
	r.Base.SetColorEnabled(enabled)
}
