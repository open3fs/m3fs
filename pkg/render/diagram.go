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

// Color constants
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

// Layout constants
const (
	DefaultRowSize               = 8
	DefaultDiagramWidth          = 70
	DefaultNodeCellWidth         = 16
	DefaultServiceBoxPadding     = 2
	DefaultTotalCellWidth        = 14
	DefaultMinServiceLines       = 6 // Minimum number of service lines to ensure consistent height
	InitialStringBuilderCapacity = 1024
	InitialMapCapacity           = 16
)

// ServiceConfig defines a service configuration for rendering
type ServiceConfig struct {
	Type  config.ServiceType
	Name  string
	Color string
}

// DiagramRenderer is responsible for rendering architecture diagrams
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

// RenderNodeBox renders a box for a node
func (r *DiagramRenderer) RenderNodeBox(sb *strings.Builder, nodeName string, services []string) {
	const cellContent = "                "
	const boxSpacing = " "

	topBorder := "+" + strings.Repeat("-", len(cellContent)) + "+"
	sb.WriteString(topBorder)
	sb.WriteString(boxSpacing)
	sb.WriteByte('\n')

	nodeNameLine := "|" + fmt.Sprintf("%s%-16s%s", r.GetColorCode(ColorCyan),
		nodeName, r.GetColorReset()) + "|"
	sb.WriteString(nodeNameLine)
	sb.WriteString(boxSpacing)
	sb.WriteByte('\n')

	for _, service := range services {
		serviceLine := "|" + fmt.Sprintf("  %s%s%s%s",
			r.GetColorCode(ColorGreen),
			service,
			r.GetColorReset(),
			strings.Repeat(" ", len(cellContent)-len(service)-2)) + "|"
		sb.WriteString(serviceLine)
		sb.WriteString(boxSpacing)
		sb.WriteByte('\n')
	}

	// Render bottom border
	bottomBorder := "+" + strings.Repeat("-", len(cellContent)) + "+"
	sb.WriteString(bottomBorder)
	sb.WriteString(boxSpacing)
	sb.WriteByte('\n')
}

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

// GetStringBuilder creates a new strings.Builder
func (r *DiagramRenderer) GetStringBuilder() *strings.Builder {
	sb := &strings.Builder{}
	sb.Grow(InitialStringBuilderCapacity)
	return sb
}

// PutStringBuilder resets the strings.Builder (no-op for GC)
func (r *DiagramRenderer) PutStringBuilder(sb *strings.Builder) {
	// 简单实现，只需重置字符串构建器，让垃圾回收处理内存
	sb.Reset()
}

// getServiceColor returns the appropriate color for a service based on its name
func (r *DiagramRenderer) getServiceColor(service string) string {
	switch {
	case strings.Contains(service, "/mnt/") || strings.Contains(service, "/mount/"):
		return ColorPink
	case strings.Contains(service, "storage"):
		return ColorYellow
	case strings.Contains(service, "fdb"):
		return ColorBlue
	case strings.Contains(service, "meta"):
		return ColorPink
	case strings.Contains(service, "mgmtd"):
		return ColorPurple
	case strings.Contains(service, "monitor"):
		return ColorPurple
	case strings.Contains(service, "clickhouse"):
		return ColorRed
	case strings.Contains(service, "hf3fs_fuse"):
		return ColorGreen
	default:
		return ColorWhite
	}
}

// RenderNodesRow renders a row of nodes horizontally
func (r *DiagramRenderer) RenderNodesRow(sb *strings.Builder, nodes []string,
	servicesFn func(string) []string, enforceMinHeight bool) {
	if len(nodes) == 0 {
		return
	}

	// Calculate maximum number of rows needed for each node
	maxServiceLines := 0
	nodeServices := make([][]string, len(nodes))
	for i, node := range nodes {
		services := servicesFn(node)
		nodeServices[i] = services
		if len(services) > maxServiceLines {
			maxServiceLines = len(services)
		}
	}

	// Apply minimum height if required for consistency
	if enforceMinHeight && maxServiceLines < DefaultMinServiceLines {
		maxServiceLines = DefaultMinServiceLines
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
				color := r.getServiceColor(service)

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
