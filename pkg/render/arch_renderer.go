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
	"math"
	"strings"

	"github.com/open3fs/m3fs/pkg/config"
)

// ===== Architecture Diagram Renderer =====

// ArchRenderer handles rendering architecture diagrams
type ArchRenderer struct {
	base         *DiagramRenderer
	dataProvider NodeDataProvider
}

// NewArchRenderer creates a new architecture renderer
func NewArchRenderer(baseRenderer *DiagramRenderer, dataProvider NodeDataProvider) *ArchRenderer {
	return &ArchRenderer{
		base:         baseRenderer,
		dataProvider: dataProvider,
	}
}

// SetColorEnabled enables or disables color output
func (r *ArchRenderer) SetColorEnabled(enabled bool) {
	r.base.SetColorEnabled(enabled)
}

// Generate renders the complete architecture diagram
func (r *ArchRenderer) Generate() string {
	sb := &strings.Builder{}
	sb.Grow(1024)

	clientNodes := r.dataProvider.GetClientNodes()
	storageNodes := r.dataProvider.GetRenderableNodes()

	nodeCount := int(math.Min(float64(len(clientNodes)), float64(r.base.RowSize)))

	r.renderClusterHeader(sb, nodeCount)
	r.renderClientSection(sb, clientNodes)
	r.renderNetworkSection(sb,
		r.dataProvider.GetNetworkType(),
		r.dataProvider.GetNetworkSpeed())
	r.renderStorageSection(sb, storageNodes)
	r.renderSummarySection(sb)

	return sb.String()
}

// ===== ArchRenderer Header Methods =====

// renderClusterHeader renders the diagram header
func (r *ArchRenderer) renderClusterHeader(sb *strings.Builder, nodeCount int) {
	replicationFactor := r.base.cfg.Services.Storage.ReplicationFactor
	titleText := fmt.Sprintf("Cluster: %s  replicationFactor: %d", r.base.cfg.Name, replicationFactor)

	r.base.RenderLine(sb, titleText, "")
	r.renderCustomDivider(sb, "=", nodeCount)
	sb.WriteByte('\n')
}

// renderCustomDivider renders a divider with the specified width
func (r *ArchRenderer) renderCustomDivider(sb *strings.Builder, char string, nodeCount int) {
	width := (r.base.NodeCellWidth + 3) * nodeCount
	sb.WriteString(strings.Repeat(char, width))
	sb.WriteByte('\n')
}

// renderCustomSectionHeader renders a section header with custom width
func (r *ArchRenderer) renderCustomSectionHeader(sb *strings.Builder, title string, nodeCount int) {
	r.base.RenderWithColor(sb, title, ColorCyan)
	sb.WriteByte('\n')
	r.renderCustomDivider(sb, "-", nodeCount)
}

// ===== ArchRenderer Client Section Methods =====

// renderClientSection renders the client nodes section
func (r *ArchRenderer) renderClientSection(sb *strings.Builder, clientNodes []string) {
	if len(clientNodes) == 0 {
		return
	}

	nodeCount := int(math.Min(float64(len(clientNodes)), float64(r.base.RowSize)))
	r.renderCustomSectionHeader(sb, "CLIENT NODES:", nodeCount)

	clientCount := len(clientNodes)
	r.base.lastClientNodesCount = clientCount

	for i := 0; i < clientCount; i += r.base.RowSize {
		end := i + r.base.RowSize
		if end > clientCount {
			end = clientCount
		}
		r.renderClientNodes(sb, clientNodes[i:end])
	}

	sb.WriteByte('\n')
	nodeRowWidth := (r.base.NodeCellWidth + 3) * nodeCount
	r.renderConnectionArrows(sb, nodeRowWidth, nodeCount)
	sb.WriteByte('\n')
}

// renderClientNodes renders client nodes
func (r *ArchRenderer) renderClientNodes(sb *strings.Builder, clientNodes []string) {
	if len(clientNodes) == 0 {
		return
	}

	hostMountpoint := ""
	if r.base.cfg != nil && r.base.cfg.Services.Client.HostMountpoint != "" {
		hostMountpoint = r.base.cfg.Services.Client.HostMountpoint
	} else {
		hostMountpoint = "/mnt/3fs"
	}

	mountpointStr := fmt.Sprintf("[%s]", hostMountpoint)
	fuseStr := "[hf3fs_fuse]"

	// Render top border of rectangles
	const cellContent = "                "
	const boxSpacing = " "

	for i := range clientNodes {
		sb.WriteString("+" + strings.Repeat("-", len(cellContent)) + "+")
		if i < len(clientNodes)-1 {
			sb.WriteString(boxSpacing)
		}
	}
	sb.WriteByte('\n')

	// Render node names
	for i, node := range clientNodes {
		nodeName := node
		if len(nodeName) > r.base.NodeCellWidth {
			nodeName = nodeName[:13] + "..."
		}
		sb.WriteString("|" + fmt.Sprintf("%s%-16s%s", r.base.GetColorCode(ColorCyan),
			nodeName, r.base.GetColorReset()) + "|")
		if i < len(clientNodes)-1 {
			sb.WriteString(boxSpacing)
		}
	}
	sb.WriteByte('\n')

	// Render first service line - hf3fs_fuse
	for i := range clientNodes {
		sb.WriteString("|" + fmt.Sprintf("  %s%s%s%s",
			r.base.GetColorCode(ColorGreen),
			fuseStr,
			r.base.GetColorReset(),
			strings.Repeat(" ", len(cellContent)-len(fuseStr)-2)) + "|")
		if i < len(clientNodes)-1 {
			sb.WriteString(boxSpacing)
		}
	}
	sb.WriteByte('\n')

	// Render second service line - mountpoint
	for i := range clientNodes {
		sb.WriteString("|" + fmt.Sprintf("  %s%s%s%s",
			r.base.GetColorCode(ColorPink),
			mountpointStr,
			r.base.GetColorReset(),
			strings.Repeat(" ", len(cellContent)-len(mountpointStr)-2)) + "|")
		if i < len(clientNodes)-1 {
			sb.WriteString(boxSpacing)
		}
	}
	sb.WriteByte('\n')

	// Render bottom border of rectangles
	for i := range clientNodes {
		sb.WriteString("+" + strings.Repeat("-", len(cellContent)) + "+")
		if i < len(clientNodes)-1 {
			sb.WriteString(boxSpacing)
		}
	}
	sb.WriteByte('\n')
}

// ===== ArchRenderer Connection Methods =====

// calculateArrowCount calculates the number of arrows to display
func (r *ArchRenderer) calculateArrowCount(nodeRowWidth, maxArrows int) int {
	const arrowWidth = 7 // "   ↓   "

	// If client node count is less than 8 (maxArrows param), use node count as arrow count
	if maxArrows > 0 && maxArrows < r.base.RowSize {
		return maxArrows // Return node count as arrow count
	}

	// Otherwise calculate based on width
	innerWidth := nodeRowWidth - 2
	arrowCount := innerWidth / arrowWidth

	if arrowCount <= 0 {
		return 1
	} else if arrowCount > 15 {
		return 15
	}

	if maxArrows > 0 && arrowCount > maxArrows {
		arrowCount = maxArrows
	}

	return arrowCount
}

// renderConnectionArrows renders arrows that align with the network frame
func (r *ArchRenderer) renderConnectionArrows(sb *strings.Builder, networkWidth, maxArrows int) {
	const arrowStr = "   ↓   "

	arrowCount := r.calculateArrowCount(networkWidth, maxArrows)
	totalArrowWidth := len(arrowStr) * arrowCount
	innerWidth := networkWidth - 2

	leftPadding := (innerWidth - totalArrowWidth) / 2

	// Apply adjustments for visual centering based on arrow count
	if arrowCount <= 3 {
		leftPadding += 2
	} else if arrowCount <= 6 {
		leftPadding += 5
	} else if arrowCount <= 8 {
		leftPadding += 8
	} else {
		leftPadding += 13
	}

	if leftPadding < 0 {
		leftPadding = 0
	}

	sb.WriteString(strings.Repeat(" ", leftPadding))
	sb.WriteString(strings.Repeat(arrowStr, arrowCount))
}

// ===== ArchRenderer Network Section Methods =====

// renderNetworkSection renders the network section
func (r *ArchRenderer) renderNetworkSection(sb *strings.Builder, networkType, networkSpeed string) {
	clientNodes := r.base.lastClientNodesCount

	networkText := ""
	if clientNodes == 1 {
		networkText = fmt.Sprintf("%s (%s)", networkType, networkSpeed)
	} else {
		networkText = fmt.Sprintf("%s Network (%s)", networkType, networkSpeed)
	}

	if clientNodes <= 0 {
		clientNodes = 3
	}

	actualNodeCount := clientNodes
	if actualNodeCount > r.base.RowSize {
		actualNodeCount = r.base.RowSize
	}

	nodeRowWidth := (r.base.NodeCellWidth + 3) * actualNodeCount
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
	r.base.RenderWithColor(sb, networkText, ColorBlue)
	sb.WriteString(strings.Repeat(" ", rightPadding) + "║\n")
	sb.WriteString("╚" + strings.Repeat("═", nodeRowWidth-2) + "╝\n")
}

// ===== ArchRenderer Storage Section Methods =====

// renderStorageSection renders the storage nodes section
func (r *ArchRenderer) renderStorageSection(sb *strings.Builder, storageNodes []string) {
	storageCount := len(storageNodes)
	if storageCount == 0 {
		return
	}

	firstRowCount := storageCount
	if firstRowCount > r.base.RowSize {
		firstRowCount = r.base.RowSize
	}

	storageNodeCount := int(math.Min(float64(storageCount), float64(r.base.RowSize)))
	maxClientNodes := int(math.Min(float64(r.base.lastClientNodesCount), float64(r.base.RowSize)))
	nodeRowWidth := (r.base.NodeCellWidth + 3) * maxClientNodes

	r.renderConnectionArrows(sb, nodeRowWidth, storageNodeCount)

	sb.WriteString("\n\n")
	r.renderCustomSectionHeader(sb, "STORAGE NODES:", firstRowCount)

	for i := 0; i < storageCount; i += r.base.RowSize {
		end := i + r.base.RowSize
		if end > storageCount {
			end = storageCount
		}

		if len(storageNodes[i:end]) > 0 {
			r.base.RenderNodeRow(sb, storageNodes[i:end], r.dataProvider.GetNodeServices)
		}
	}
}

// ===== ArchRenderer Summary Section Methods =====

// SummaryItem represents an item in the summary section
type SummaryItem struct {
	name  string
	count int
	color string
}

// renderSummarySection renders the summary section
func (r *ArchRenderer) renderSummarySection(sb *strings.Builder) {
	sb.WriteString("\n")
	r.base.RenderSectionHeader(sb, "CLUSTER SUMMARY:")

	serviceCounts := r.dataProvider.GetServiceNodeCounts()
	totalNodeCount := r.dataProvider.GetTotalNodeCount()

	r.renderSummaryStatRow(sb, []SummaryItem{
		{"Client Nodes", serviceCounts[config.ServiceClient], ColorGreen},
		{"Storage Nodes", serviceCounts[config.ServiceStorage], ColorYellow},
		{"FoundationDB", serviceCounts[config.ServiceFdb], ColorBlue},
		{"Meta Service", serviceCounts[config.ServiceMeta], ColorPink},
	})

	r.renderSummaryStatRow(sb, []SummaryItem{
		{"Mgmtd Service", serviceCounts[config.ServiceMgmtd], ColorPurple},
		{"Monitor Svc", serviceCounts[config.ServiceMonitor], ColorPurple},
		{"Clickhouse", serviceCounts[config.ServiceClickhouse], ColorRed},
		{"Total Nodes", totalNodeCount, ColorCyan},
	})
}

// renderSummaryStatRow renders a row of summary statistics
func (r *ArchRenderer) renderSummaryStatRow(sb *strings.Builder, items []SummaryItem) {
	for _, item := range items {
		sb.WriteString(fmt.Sprintf("%s%-14s%s %-2d  ",
			r.base.GetColorCode(item.color),
			item.name+":",
			r.base.GetColorReset(),
			item.count))
	}
	sb.WriteByte('\n')
}
