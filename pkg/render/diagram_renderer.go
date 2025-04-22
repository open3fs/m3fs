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

	for i := range nodes {
		sb.WriteString("+" + strings.Repeat("-", len(cellContent)) + "+")
		if i < len(nodes)-1 {
			sb.WriteString(boxSpacing)
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
			sb.WriteString(boxSpacing)
		}
	}
	sb.WriteByte('\n')

	for serviceIdx := 0; serviceIdx < maxServiceLines; serviceIdx++ {
		for nodeIdx, services := range nodeServices {
			if serviceIdx < len(services) {
				service := services[serviceIdx]
				color := r.GetServiceColor(service)

				sb.WriteString("|" + fmt.Sprintf("  %s%s%s%s",
					r.GetColorCode(color),
					service,
					r.GetColorReset(),
					strings.Repeat(" ", len(cellContent)-len(service)-2)) + "|")
			} else {
				sb.WriteString("|" + strings.Repeat(" ", len(cellContent)) + "|")
			}
			if nodeIdx < len(nodes)-1 {
				sb.WriteString(boxSpacing)
			}
		}
		sb.WriteByte('\n')
	}

	for i := range nodes {
		sb.WriteString("+" + strings.Repeat("-", len(cellContent)) + "+")
		if i < len(nodes)-1 {
			sb.WriteString(boxSpacing)
		}
	}
	sb.WriteByte('\n')
}
