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
	"strings"
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

	// Gather data
	clientNodes := r.dataProvider.GetClientNodes()
	storageNodes := r.dataProvider.GetRenderableNodes()
	serviceCounts := r.dataProvider.GetServiceNodeCounts()
	totalNodeCount := r.dataProvider.GetTotalNodeCount()
	networkType := r.dataProvider.GetNetworkType()
	networkSpeed := r.dataProvider.GetNetworkSpeed()

	// Calculate dimensions
	clientNodeCount := len(clientNodes)
	storageNodeCount := len(storageNodes)
	clientDisplayCount := r.base.CalculateMaxNodeCount(clientNodeCount)
	storageDisplayCount := r.base.CalculateMaxNodeCount(storageNodeCount)

	// Calculate width
	clientSectionWidth := r.base.CalculateNodeRowWidth(clientDisplayCount) - 2

	// Render diagram
	r.base.RenderHeader(sb, clientSectionWidth)

	if clientNodeCount > 0 {
		r.base.RenderClientSection(sb, clientNodes)
	}

	if clientNodeCount > 0 && storageNodeCount > 0 {
		r.base.RenderNetworkConnection(
			sb,
			networkType,
			networkSpeed,
			clientDisplayCount,
			storageDisplayCount,
		)
	}

	if storageNodeCount > 0 {
		r.base.RenderStorageSection(sb, storageNodes, r.dataProvider.GetNodeServices)
	}

	r.base.RenderSummarySection(sb, serviceCounts, totalNodeCount)

	return sb.String()
}
