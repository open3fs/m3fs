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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open3fs/m3fs/pkg/config"
)

// TestDrawClusterArchitecture tests the no-color option in the architecture diagram generation
func TestDrawClusterArchitecture(t *testing.T) {
	// We'll directly test the ArchitectureDiagramGenerator since we can't mock drawClusterArchitecture
	cfg := createTestConfig()

	// Explicit use of config package to avoid unused import error
	var _ config.NetworkType = cfg.NetworkType

	t.Run("WithColor", func(t *testing.T) {
		generator := NewArchitectureDiagramGenerator(cfg)
		// Default should be with color
		diagram := generator.Generate()
		assert.Contains(t, diagram, "\033[", "Output should contain color codes")
		assert.Contains(t, diagram, "Cluster: test-cluster")
	})

	t.Run("WithoutColor", func(t *testing.T) {
		generator := NewArchitectureDiagramGenerator(cfg)
		generator.SetColorEnabled(false)
		diagram := generator.Generate()
		assert.NotContains(t, diagram, "\033[", "Output should not contain color codes")
		assert.Contains(t, diagram, "Cluster: test-cluster")
	})
}
