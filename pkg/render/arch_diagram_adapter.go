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

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/utils"
)

// ArchDiagramAdapter is an adapter that connects a NodeDataProvider with an ArchDiagramRenderer
type ArchDiagramAdapter struct {
	dataProvider NodeDataProvider
	renderer     *ArchDiagramRenderer
}

// NewArchDiagramAdapter creates a new ArchDiagramAdapter
func NewArchDiagramAdapter(dataProvider NodeDataProvider, renderer *ArchDiagramRenderer) *ArchDiagramAdapter {
	return &ArchDiagramAdapter{
		dataProvider: dataProvider,
		renderer:     renderer,
	}
}

// Generate generates the architecture diagram
func (a *ArchDiagramAdapter) Generate() string {
	sb := &strings.Builder{}
	sb.Grow(1024)

	clientNodes := a.dataProvider.GetClientNodes()
	renderableNodes := a.dataProvider.GetRenderableNodes()

	nodeCount := utils.Min(len(clientNodes), a.renderer.Base.RowSize)
	a.renderer.RenderHeader(sb, nodeCount)

	a.renderer.RenderClientSection(sb, clientNodes)

	a.renderer.RenderNetworkSection(sb, a.dataProvider.GetNetworkType(), a.dataProvider.GetNetworkSpeed())

	a.renderer.RenderStorageSection(sb, renderableNodes, a.dataProvider.GetNodeServices)

	a.renderer.RenderSummarySection(sb, a.getServiceNodeCountsFunc(), a.dataProvider.GetTotalNodeCount())

	return sb.String()
}

// getServiceNodeCountsFunc returns a function that provides service node counts
func (a *ArchDiagramAdapter) getServiceNodeCountsFunc() ServiceCountFunc {
	return func() map[config.ServiceType]int {
		return a.dataProvider.GetServiceNodeCounts()
	}
}

// SetColorEnabled enables or disables color output
func (a *ArchDiagramAdapter) SetColorEnabled(enabled bool) {
	if a.renderer != nil {
		a.renderer.SetColorEnabled(enabled)
	}
}
