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

// ===== Constants =====

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
)

// ===== Data Types =====

// ServiceConfig defines a service configuration for rendering
type ServiceConfig struct {
	Type  config.ServiceType
	Name  string
	Color string
}

// NodeServicesFunc returns services for a node
type NodeServicesFunc func(nodeName string) []string

// NodeDataProvider provides node data for architecture diagrams
type NodeDataProvider interface {
	GetClientNodes() []string
	GetRenderableNodes() []string
	GetNodeServices(nodeName string) []string
	GetServiceNodeCounts() map[config.ServiceType]int
	GetTotalNodeCount() int
	GetNetworkSpeed() string
	GetNetworkType() string
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
	// Strip brackets if present for lookup
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

// ===== Basic Rendering Methods =====

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
func (r *DiagramRenderer) RenderHeader(sb *strings.Builder) {
	r.RenderLine(sb, "Cluster: "+r.cfg.Name, "")
	r.RenderDivider(sb, "=", r.Width)
	sb.WriteByte('\n')
}

// RenderSectionHeader renders a section header
func (r *DiagramRenderer) RenderSectionHeader(sb *strings.Builder, title string) {
	r.RenderWithColor(sb, title, ColorCyan)
	sb.WriteByte('\n')
	r.RenderDivider(sb, "-", r.Width)
}

// ===== Node Rendering Methods =====

// RenderNodeRow renders a row of nodes
func (r *DiagramRenderer) RenderNodeRow(sb *strings.Builder, nodes []string, servicesFn func(string) []string) {
	if len(nodes) == 0 {
		return
	}

	// Calculate maximum number of service lines
	maxServiceLines := DefaultMinServiceLines
	nodeServices := make([][]string, len(nodes))

	for i, node := range nodes {
		services := servicesFn(node)
		nodeServices[i] = services
		if len(services) > maxServiceLines {
			maxServiceLines = len(services)
		}
	}

	const cellContent = "                "
	const boxSpacing = " "

	// Render top border
	for i := range nodes {
		sb.WriteString("+" + strings.Repeat("-", len(cellContent)) + "+")
		if i < len(nodes)-1 {
			sb.WriteString(boxSpacing)
		}
	}
	sb.WriteByte('\n')

	// Render node names
	for i, node := range nodes {
		nodeName := node
		if len(nodeName) > r.NodeCellWidth {
			nodeName = nodeName[:13] + "..."
		}
		sb.WriteString("|" + fmt.Sprintf("%s%-16s%s", r.GetColorCode(ColorCyan),
			nodeName, r.GetColorReset()) + "|")
		if i < len(nodes)-1 {
			sb.WriteString(boxSpacing)
		}
	}
	sb.WriteByte('\n')

	// Render services, ensuring each node has the same number of rows
	for serviceIdx := 0; serviceIdx < maxServiceLines; serviceIdx++ {
		for nodeIdx, services := range nodeServices {
			if serviceIdx < len(services) {
				service := services[serviceIdx]

				// Choose color based on service type
				color := r.GetServiceColor(service)

				sb.WriteString("|" + fmt.Sprintf("  %s%s%s%s",
					r.GetColorCode(color),
					service,
					r.GetColorReset(),
					strings.Repeat(" ", len(cellContent)-len(service)-2)) + "|")
			} else {
				// Add empty line for nodes with fewer services
				sb.WriteString("|" + strings.Repeat(" ", len(cellContent)) + "|")
			}
			if nodeIdx < len(nodes)-1 {
				sb.WriteString(boxSpacing)
			}
		}
		sb.WriteByte('\n')
	}

	// Render bottom border
	for i := range nodes {
		sb.WriteString("+" + strings.Repeat("-", len(cellContent)) + "+")
		if i < len(nodes)-1 {
			sb.WriteString(boxSpacing)
		}
	}
	sb.WriteByte('\n')
}

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

	// Use max row size or client node count, whichever is smaller
	nodeCount := int(math.Min(float64(len(clientNodes)), float64(r.base.RowSize)))

	// Generate diagram sections
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

	r.base.RenderNodeRow(sb, clientNodes, func(node string) []string {
		return []string{
			"[hf3fs_fuse]",
			mountpointStr,
		}
	})
}

// ===== ArchRenderer Connection Methods =====

// calculateArrowCount calculates the number of arrows to display
func (r *ArchRenderer) calculateArrowCount(nodeRowWidth, maxArrows int) int {
	const arrowWidth = 7 // "   ↓   "

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
	const arrowStr = "   ↓   " // Arrow string with equal spaces on both sides

	arrowCount := r.calculateArrowCount(networkWidth, maxArrows)
	totalArrowWidth := len(arrowStr) * arrowCount
	innerWidth := networkWidth - 2
	leftPadding := (innerWidth - totalArrowWidth) / 2

	// Adjust padding for visual alignment
	leftPadding += 2
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

	// Optimize network text for display
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

// renderSummarySection renders the summary section
func (r *ArchRenderer) renderSummarySection(sb *strings.Builder) {
	sb.WriteString("\n")
	r.base.RenderSectionHeader(sb, "CLUSTER SUMMARY:")

	serviceCounts := r.dataProvider.GetServiceNodeCounts()
	totalNodeCount := r.dataProvider.GetTotalNodeCount()

	// First row of stats
	r.renderSummaryStatRow(sb, []SummaryItem{
		{"Client Nodes", serviceCounts[config.ServiceClient], ColorGreen},
		{"Storage Nodes", serviceCounts[config.ServiceStorage], ColorYellow},
		{"FoundationDB", serviceCounts[config.ServiceFdb], ColorBlue},
		{"Meta Service", serviceCounts[config.ServiceMeta], ColorPink},
	})

	// Second row of stats
	r.renderSummaryStatRow(sb, []SummaryItem{
		{"Mgmtd Service", serviceCounts[config.ServiceMgmtd], ColorPurple},
		{"Monitor Svc", serviceCounts[config.ServiceMonitor], ColorPurple},
		{"Clickhouse", serviceCounts[config.ServiceClickhouse], ColorRed},
		{"Total Nodes", totalNodeCount, ColorCyan},
	})
}

// SummaryItem represents an item in the summary section
type SummaryItem struct {
	name  string
	count int
	color string
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

// ===== Data Provider =====

// ClusterDataProvider provides cluster node data
type ClusterDataProvider struct {
	getServiceNodeCountsFunc func() map[config.ServiceType]int
	getClientNodesFunc       func() []string
	getRenderableNodesFunc   func() []string
	getNodeServicesFunc      func(string) []string
	getTotalNodeCountFunc    func() int
	getNetworkSpeedFunc      func() string
	getNetworkTypeFunc       func() string
}

// NewClusterDataProvider creates a new ClusterDataProvider
func NewClusterDataProvider(
	serviceNodeCountsFunc func() map[config.ServiceType]int,
	clientNodesFunc func() []string,
	renderableNodesFunc func() []string,
	nodeServicesFunc func(string) []string,
	totalNodeCountFunc func() int,
	networkSpeedFunc func() string,
	networkTypeFunc func() string,
) *ClusterDataProvider {
	return &ClusterDataProvider{
		getServiceNodeCountsFunc: serviceNodeCountsFunc,
		getClientNodesFunc:       clientNodesFunc,
		getRenderableNodesFunc:   renderableNodesFunc,
		getNodeServicesFunc:      nodeServicesFunc,
		getTotalNodeCountFunc:    totalNodeCountFunc,
		getNetworkSpeedFunc:      networkSpeedFunc,
		getNetworkTypeFunc:       networkTypeFunc,
	}
}

// ===== Data Provider Interface Methods =====

// GetClientNodes returns all client nodes
func (p *ClusterDataProvider) GetClientNodes() []string {
	return p.getClientNodesFunc()
}

// GetRenderableNodes returns nodes to render in the diagram
func (p *ClusterDataProvider) GetRenderableNodes() []string {
	return p.getRenderableNodesFunc()
}

// GetNodeServices returns services running on a node
func (p *ClusterDataProvider) GetNodeServices(nodeName string) []string {
	return p.getNodeServicesFunc(nodeName)
}

// GetServiceNodeCounts returns node counts by service type
func (p *ClusterDataProvider) GetServiceNodeCounts() map[config.ServiceType]int {
	return p.getServiceNodeCountsFunc()
}

// GetTotalNodeCount returns the total number of nodes
func (p *ClusterDataProvider) GetTotalNodeCount() int {
	return p.getTotalNodeCountFunc()
}

// GetNetworkSpeed returns the network speed
func (p *ClusterDataProvider) GetNetworkSpeed() string {
	return p.getNetworkSpeedFunc()
}

// GetNetworkType returns the network type
func (p *ClusterDataProvider) GetNetworkType() string {
	return p.getNetworkTypeFunc()
}
