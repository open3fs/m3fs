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
	"fmt"
	"math"
	"strings"

	"github.com/open3fs/m3fs/pkg/config"
)

// Color constants for terminal output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorPink   = "\033[38;5;219m"
)

// Layout constants for diagram rendering
const (
	DefaultRowSize         = 8
	DefaultDiagramWidth    = 70
	DefaultNodeCellWidth   = 16
	DefaultTotalCellWidth  = 14
	DefaultMinServiceLines = 6

	// Box layout constants
	CellContent = "                " // 16 spaces
	BoxSpacing  = " "                // 1 space
)

// ServiceConfig defines a service configuration for rendering
type ServiceConfig struct {
	Type  config.ServiceType
	Name  string
	Color string
}

// NodeServicesFunc returns services for a node
type NodeServicesFunc func(nodeName string) []string

// SummaryItem represents an item in the summary section
type SummaryItem struct {
	Name  string
	Count int
	Color string
}

// NewSummaryItem creates a new SummaryItem with the given parameters
func NewSummaryItem(name string, count int, color string) SummaryItem {
	return SummaryItem{
		Name:  name,
		Count: count,
		Color: color,
	}
}

// ===== Base Diagram Renderer =====

// DiagramRenderer provides base rendering functionality for diagrams
type DiagramRenderer struct {
	cfg           *config.Config
	ColorEnabled  bool
	Width         int
	RowSize       int
	NodeCellWidth int

	ServiceConfigs []ServiceConfig

	lastClientNodesCount int
}

// NewDiagramRenderer creates a new diagram renderer
func NewDiagramRenderer(cfg *config.Config) *DiagramRenderer {
	if cfg == nil {
		cfg = &config.Config{Name: "default"}
	}

	return &DiagramRenderer{
		cfg:            cfg,
		ColorEnabled:   true,
		Width:          DefaultDiagramWidth,
		RowSize:        DefaultRowSize,
		NodeCellWidth:  DefaultNodeCellWidth,
		ServiceConfigs: getDefaultServiceConfigs(),
	}
}

// getDefaultServiceConfigs returns default service configurations
func getDefaultServiceConfigs() []ServiceConfig {
	return []ServiceConfig{
		{config.ServiceStorage, "storage", ColorYellow},
		{config.ServiceFdb, "foundationdb", ColorBlue},
		{config.ServiceMeta, "meta", ColorPink},
		{config.ServiceMgmtd, "mgmtd", ColorPurple},
		{config.ServiceMonitor, "monitor", ColorPurple},
		{config.ServiceClickhouse, "clickhouse", ColorRed},
	}
}

// SetColorEnabled enables or disables color output
func (r *DiagramRenderer) SetColorEnabled(enabled bool) {
	r.ColorEnabled = enabled
}

// ===== Color Management Methods =====

// GetColorCode returns the color code if colors are enabled
func (r *DiagramRenderer) GetColorCode(color string) string {
	if !r.ColorEnabled {
		return ""
	}
	return color
}

// GetColorReset returns the color reset code
func (r *DiagramRenderer) GetColorReset() string {
	return r.GetColorCode(ColorReset)
}

// GetServiceColor returns the color for a service
func (r *DiagramRenderer) GetServiceColor(service string) string {
	cleanService := strings.Trim(service, "[]")

	switch {
	case strings.Contains(cleanService, "storage"):
		return ColorYellow
	case strings.Contains(cleanService, "foundationdb"):
		return ColorBlue
	case strings.Contains(cleanService, "meta"):
		return ColorPink
	case strings.Contains(cleanService, "mgmtd"):
		return ColorPurple
	case strings.Contains(cleanService, "monitor"):
		return ColorPurple
	case strings.Contains(cleanService, "clickhouse"):
		return ColorRed
	case strings.Contains(cleanService, "hf3fs_fuse"):
		return ColorGreen
	case strings.Contains(cleanService, "/mnt/") || strings.Contains(cleanService, "/mount/"):
		return ColorPink
	default:
		return ColorGreen
	}
}

// RenderWithColor renders text with the specified color
func (r *DiagramRenderer) RenderWithColor(sb *strings.Builder, text string, color string) {
	if !r.ColorEnabled || color == "" {
		sb.WriteString(text)
		return
	}
	sb.WriteString(color)
	sb.WriteString(text)
	sb.WriteString(ColorReset)
}

// ===== Layout Calculation Methods =====

// CalculateNodeRowWidth calculates the width of a row of nodes
func (r *DiagramRenderer) CalculateNodeRowWidth(nodeCount int) int {
	if nodeCount <= 0 {
		nodeCount = 1
	}

	actualNodeCount := nodeCount
	if actualNodeCount > r.RowSize {
		actualNodeCount = r.RowSize
	}

	return (r.NodeCellWidth + 3) * actualNodeCount
}

// CalculateCenteredTextPadding calculates the left and right padding to center text in a given width
func (r *DiagramRenderer) CalculateCenteredTextPadding(totalWidth int, textLength int) (leftPadding, rightPadding int) {
	totalInnerWidth := totalWidth - 2
	leftPadding = (totalInnerWidth - textLength) / 2
	rightPadding = totalInnerWidth - textLength - leftPadding

	if leftPadding < 0 {
		leftPadding = 0
	}
	if rightPadding < 0 {
		rightPadding = 0
	}

	return leftPadding, rightPadding
}

// CalculateMaxNodeCount calculates the actual number of nodes to display, respecting row size limit
func (r *DiagramRenderer) CalculateMaxNodeCount(nodeCount int) int {
	if nodeCount <= 0 {
		return 0
	}
	return int(math.Min(float64(nodeCount), float64(r.RowSize)))
}

// CalculateArrowCount calculates the number of arrows to display
func (r *DiagramRenderer) CalculateArrowCount(width, maxArrows int) int {
	const arrowWidth = 7

	if maxArrows > 0 && maxArrows < r.RowSize {
		return maxArrows
	}

	innerWidth := width - 2
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

// RenderLine renders a line of text with optional color
func (r *DiagramRenderer) RenderLine(sb *strings.Builder, text string, color string) {
	if color != "" {
		r.RenderWithColor(sb, text, color)
	} else {
		sb.WriteString(text)
	}
	sb.WriteByte('\n')
}

// RenderDivider renders a divider line
func (r *DiagramRenderer) RenderDivider(sb *strings.Builder, char string, width int) {
	if width <= 0 {
		return
	}
	sb.WriteString(strings.Repeat(char, width))
	sb.WriteByte('\n')
}

// RenderHeader renders the diagram header
// If replicationFactor is provided, it will be included in the title
func (r *DiagramRenderer) RenderHeader(sb *strings.Builder, customWidth ...int) {
	titleText := "Cluster: " + r.cfg.Name

	if r.cfg != nil && r.cfg.Services.Storage.ReplicationFactor > 0 {
		titleText = fmt.Sprintf("Cluster: %s  replicationFactor: %d",
			r.cfg.Name, r.cfg.Services.Storage.ReplicationFactor)
	}

	r.RenderLine(sb, titleText, "")

	width := r.Width
	if len(customWidth) > 0 && customWidth[0] > 0 {
		width = customWidth[0]
	}

	r.RenderDivider(sb, "=", width)
	sb.WriteByte('\n')
}

// RenderSectionHeader renders a section header
// If customWidth > 0, uses that width instead of the default r.Width
func (r *DiagramRenderer) RenderSectionHeader(sb *strings.Builder, title string, customWidth ...int) {
	r.RenderWithColor(sb, title, ColorCyan)
	sb.WriteByte('\n')

	width := r.Width
	if len(customWidth) > 0 && customWidth[0] > 0 {
		width = customWidth[0]
	}

	r.RenderDivider(sb, "-", width)
}

// RenderNodeRow renders a row of nodes
func (r *DiagramRenderer) RenderNodeRow(
	sb *strings.Builder,
	nodes []string,
	servicesFn func(string) []string,
	fixedServices ...[]string,
) {
	if len(nodes) == 0 {
		return
	}

	for i := range nodes {
		sb.WriteString("+" + strings.Repeat("-", len(CellContent)) + "+")
		if i < len(nodes)-1 {
			sb.WriteString(BoxSpacing)
		}
	}
	sb.WriteByte('\n')

	for i, node := range nodes {
		nodeName := node
		if len(nodeName) > r.NodeCellWidth {
			nodeName = nodeName[:13] + "..."
		}
		sb.WriteString("|" + fmt.Sprintf("%s%-16s%s", r.GetColorCode(ColorCyan),
			nodeName, r.GetColorReset()) + "|")
		if i < len(nodes)-1 {
			sb.WriteString(BoxSpacing)
		}
	}
	sb.WriteByte('\n')

	if len(fixedServices) > 0 {
		for _, service := range fixedServices[0] {
			color := r.GetServiceColor(service)

			for i := range nodes {
				sb.WriteString("|" + fmt.Sprintf("  %s%s%s%s",
					r.GetColorCode(color),
					service,
					r.GetColorReset(),
					strings.Repeat(" ", len(CellContent)-len(service)-2)) + "|")
				if i < len(nodes)-1 {
					sb.WriteString(BoxSpacing)
				}
			}
			sb.WriteByte('\n')
		}
	} else if servicesFn != nil {
		maxServiceLines := DefaultMinServiceLines
		nodeServices := make([][]string, len(nodes))

		for i, node := range nodes {
			services := servicesFn(node)
			nodeServices[i] = services
			if len(services) > maxServiceLines {
				maxServiceLines = len(services)
			}
		}

		for serviceIdx := 0; serviceIdx < maxServiceLines; serviceIdx++ {
			for nodeIdx, services := range nodeServices {
				if serviceIdx < len(services) {
					service := services[serviceIdx]
					color := r.GetServiceColor(service)

					sb.WriteString("|" + fmt.Sprintf("  %s%s%s%s",
						r.GetColorCode(color),
						service,
						r.GetColorReset(),
						strings.Repeat(" ", len(CellContent)-len(service)-2)) + "|")
				} else {
					sb.WriteString("|" + strings.Repeat(" ", len(CellContent)) + "|")
				}
				if nodeIdx < len(nodes)-1 {
					sb.WriteString(BoxSpacing)
				}
			}
			sb.WriteByte('\n')
		}
	} else {
		for i := range nodes {
			sb.WriteString("|" + strings.Repeat(" ", len(CellContent)) + "|")
			if i < len(nodes)-1 {
				sb.WriteString(BoxSpacing)
			}
		}
		sb.WriteByte('\n')
	}

	for i := range nodes {
		sb.WriteString("+" + strings.Repeat("-", len(CellContent)) + "+")
		if i < len(nodes)-1 {
			sb.WriteString(BoxSpacing)
		}
	}
	sb.WriteByte('\n')
}

// RenderNodesWithPaging renders nodes with pagination support
// It handles breaking down large node lists into pages that fit within RowSize
func (r *DiagramRenderer) RenderNodesWithPaging(
	sb *strings.Builder,
	nodes []string,
	servicesFn func(string) []string,
	fixedServices ...[]string,
) {
	nodeCount := len(nodes)

	for i := 0; i < nodeCount; i += r.RowSize {
		end := i + r.RowSize
		if end > nodeCount {
			end = nodeCount
		}

		if len(nodes[i:end]) > 0 {
			r.RenderNodeRow(sb, nodes[i:end], servicesFn, fixedServices...)
		}
	}
}

// RenderNodeSection renders a complete node section with title and nodes
func (r *DiagramRenderer) RenderNodeSection(
	sb *strings.Builder,
	title string,
	nodes []string,
	servicesFn func(string) []string,
	fixedServices ...[]string,
) {
	if len(nodes) == 0 {
		return
	}

	nodeCount := r.CalculateMaxNodeCount(len(nodes))
	width := r.CalculateNodeRowWidth(nodeCount) - 2
	r.RenderSectionHeader(sb, title, width)
	r.RenderNodesWithPaging(sb, nodes, servicesFn, fixedServices...)
}

// RenderClientSection renders the client nodes section
func (r *DiagramRenderer) RenderClientSection(sb *strings.Builder, clientNodes []string) {
	if len(clientNodes) == 0 {
		return
	}

	r.lastClientNodesCount = len(clientNodes)
	clientServices := r.GetClientServices()
	r.RenderNodeSection(sb, "CLIENT NODES:", clientNodes, nil, clientServices)
}

// RenderStorageSection renders the storage nodes section
func (r *DiagramRenderer) RenderStorageSection(
	sb *strings.Builder,
	storageNodes []string,
	servicesFn func(string) []string,
) {
	if len(storageNodes) == 0 {
		return
	}

	r.RenderNodeSection(sb, "STORAGE NODES:", storageNodes, servicesFn)
}

// RenderConnectionArrows renders arrows that align with the network frame
func (r *DiagramRenderer) RenderConnectionArrows(sb *strings.Builder, networkWidth, maxArrows int) {
	const arrowStr = "   ↓   "

	arrowCount := r.CalculateArrowCount(networkWidth, maxArrows)
	totalArrowWidth := len(arrowStr) * arrowCount

	leftPadding, _ := r.CalculateCenteredTextPadding(networkWidth, totalArrowWidth)

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

// RenderNetworkBox renders a network box with centered text
func (r *DiagramRenderer) RenderNetworkBox(sb *strings.Builder, networkType, networkSpeed string, nodeCount int) {
	networkText := ""
	if nodeCount == 1 {
		networkText = fmt.Sprintf("%s (%s)", networkType, networkSpeed)
	} else {
		networkText = fmt.Sprintf("%s Network (%s)", networkType, networkSpeed)
	}

	if nodeCount <= 0 {
		nodeCount = 3
	}

	nodeRowWidth := r.CalculateNodeRowWidth(nodeCount)
	leftPadding, rightPadding := r.CalculateCenteredTextPadding(nodeRowWidth, len(networkText))

	sb.WriteString("╔" + strings.Repeat("═", nodeRowWidth-2) + "╗\n")
	sb.WriteString("║" + strings.Repeat(" ", leftPadding))
	r.RenderWithColor(sb, networkText, ColorBlue)
	sb.WriteString(strings.Repeat(" ", rightPadding) + "║\n")
	sb.WriteString("╚" + strings.Repeat("═", nodeRowWidth-2) + "╝\n")
}

// RenderNetworkConnection renders the full network connection, including arrows and network box
func (r *DiagramRenderer) RenderNetworkConnection(
	sb *strings.Builder,
	networkType, networkSpeed string,
	sourceNodeCount, targetNodeCount int,
) {
	nodeRowWidth := r.CalculateNodeRowWidth(sourceNodeCount)

	r.RenderConnectionArrows(sb, nodeRowWidth, targetNodeCount)
	sb.WriteByte('\n')

	r.RenderNetworkBox(sb, networkType, networkSpeed, sourceNodeCount)

	r.RenderConnectionArrows(sb, nodeRowWidth, targetNodeCount)
	sb.WriteString("\n\n")
}

// ===== Client Service Methods =====

// GetClientServices returns the standard client services for rendering
func (r *DiagramRenderer) GetClientServices() []string {
	hostMountpoint := "/mnt/3fs"
	if r.cfg != nil && r.cfg.Services.Client.HostMountpoint != "" {
		hostMountpoint = r.cfg.Services.Client.HostMountpoint
	}

	return []string{
		"[hf3fs_fuse]",
		fmt.Sprintf("[%s]", hostMountpoint),
	}
}

// ===== Summary Rendering Methods =====

// RenderSummaryStatRow renders a row of summary statistics
func (r *DiagramRenderer) RenderSummaryStatRow(sb *strings.Builder, items []SummaryItem) {
	for _, item := range items {
		sb.WriteString(fmt.Sprintf("%s%-14s%s %-2d  ",
			r.GetColorCode(item.Color),
			item.Name+":",
			r.GetColorReset(),
			item.Count))
	}
	sb.WriteByte('\n')
}

// RenderSummary renders a summary section with the given title and items
func (r *DiagramRenderer) RenderSummary(sb *strings.Builder, title string, items [][]SummaryItem) {
	sb.WriteString("\n")
	r.RenderSectionHeader(sb, title)

	for _, row := range items {
		r.RenderSummaryStatRow(sb, row)
	}
}

// RenderSummarySection renders the summary section with service counts
func (r *DiagramRenderer) RenderSummarySection(
	sb *strings.Builder,
	serviceCounts map[config.ServiceType]int,
	totalNodeCount int,
) {
	summaryRows := [][]SummaryItem{
		{
			NewSummaryItem("Client Nodes", serviceCounts[config.ServiceClient], ColorGreen),
			NewSummaryItem("Storage Nodes", serviceCounts[config.ServiceStorage], ColorYellow),
			NewSummaryItem("FoundationDB", serviceCounts[config.ServiceFdb], ColorBlue),
			NewSummaryItem("Meta Service", serviceCounts[config.ServiceMeta], ColorPink),
		},
		{
			NewSummaryItem("Mgmtd Service", serviceCounts[config.ServiceMgmtd], ColorPurple),
			NewSummaryItem("Monitor Svc", serviceCounts[config.ServiceMonitor], ColorPurple),
			NewSummaryItem("Clickhouse", serviceCounts[config.ServiceClickhouse], ColorRed),
			NewSummaryItem("Total Nodes", totalNodeCount, ColorCyan),
		},
	}

	r.RenderSummary(sb, "CLUSTER SUMMARY:", summaryRows)
}
