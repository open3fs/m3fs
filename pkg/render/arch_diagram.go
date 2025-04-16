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

// CalculateArrowCount calculates the number of arrows to display
func (r *ArchDiagramRenderer) CalculateArrowCount(nodeCount int) int {
	if nodeCount <= 0 {
		return 1
	} else if nodeCount > 15 {
		return 15
	}
	return nodeCount
}

// RenderArrows renders arrows for the specified count
func (r *ArchDiagramRenderer) RenderArrows(sb *strings.Builder, count int) {
	if count <= 0 {
		return
	}

	const arrowStr = "  ↓ "
	totalLen := len(arrowStr) * count

	arrowBuilder := r.Base.GetStringBuilder()
	arrowBuilder.Grow(totalLen)

	for i := 0; i < count; i++ {
		arrowBuilder.WriteString(arrowStr)
	}

	sb.WriteString(arrowBuilder.String())
	r.Base.PutStringBuilder(arrowBuilder)
}

// RenderNetworkSection renders the network section
func (r *ArchDiagramRenderer) RenderNetworkSection(sb *strings.Builder, networkType string, networkSpeed string) {
	networkText := fmt.Sprintf(" %s Network (%s) ", networkType, networkSpeed)
	rightPadding := r.Base.Width - 2 - len(networkText)

	sb.WriteString("╔" + strings.Repeat("═", r.Base.Width-2) + "╗\n")
	sb.WriteString("║")
	r.Base.RenderWithColor(sb, networkText, ColorBlue)
	sb.WriteString(strings.Repeat(" ", rightPadding) + "║\n")
	sb.WriteString("╚" + strings.Repeat("═", r.Base.Width-2) + "╝\n")
}

// RenderHeader renders the diagram header by delegating to the base renderer
func (r *ArchDiagramRenderer) RenderHeader(sb *strings.Builder) {
	r.Base.RenderHeader(sb)
}

// RenderClientSection renders the client nodes section
func (r *ArchDiagramRenderer) RenderClientSection(sb *strings.Builder, clientNodes []string) {
	r.Base.RenderSectionHeader(sb, "CLIENT NODES:")

	clientCount := len(clientNodes)
	for i := 0; i < clientCount; i += r.Base.RowSize {
		end := i + r.Base.RowSize
		if end > clientCount {
			end = clientCount
		}
		r.RenderClientNodes(sb, clientNodes[i:end])
	}

	sb.WriteByte('\n')
	r.RenderArrows(sb, r.CalculateArrowCount(len(clientNodes)))
	sb.WriteByte('\n')
}

// RenderClientNodes renders client nodes
func (r *ArchDiagramRenderer) RenderClientNodes(sb *strings.Builder, clientNodes []string) {
	if len(clientNodes) == 0 {
		return
	}

	r.Base.RenderNodesRow(sb, clientNodes, func(node string) []string {
		return []string{"[hf3fs_fuse]"}
	}, false)
}

// RenderStorageSection renders the storage nodes section
func (r *ArchDiagramRenderer) RenderStorageSection(
	sb *strings.Builder,
	storageNodes []string,
	nodeServicesFunc NodeServicesFunc,
) {
	r.RenderArrows(sb, r.CalculateArrowCount(len(storageNodes)))
	sb.WriteString("\n\n")

	r.Base.RenderSectionHeader(sb, "STORAGE NODES:")

	storageCount := len(storageNodes)
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

	// First row
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

	// Second row
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
