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

package utils

import (
	"sync"
)

// NodeResult represents the result of node processing
type NodeResult struct {
	Index     int
	NodeName  string
	IsStorage bool
}

// NodeResultPool provides a reuse pool for NodeResult objects
var NodeResultPool = sync.Pool{
	New: func() any {
		return &NodeResult{}
	},
}

// GetNodeResult gets a NodeResult from the object pool
func GetNodeResult() *NodeResult {
	return NodeResultPool.Get().(*NodeResult)
}

// PutNodeResult returns a NodeResult to the object pool
func PutNodeResult(nr *NodeResult) {
	if nr == nil {
		return
	}

	// Reset fields to zero values to prevent data leaks
	nr.Index = 0
	nr.NodeName = ""
	nr.IsStorage = false

	NodeResultPool.Put(nr)
}

// StringMapPool provides a reuse pool for map[string]struct{} objects
var StringMapPool = sync.Pool{
	New: func() any {
		return make(map[string]struct{}, 16)
	},
}

// GetStringMap gets a map[string]struct{} from the object pool
func GetStringMap() map[string]struct{} {
	return StringMapPool.Get().(map[string]struct{})
}

// PutStringMap returns a map[string]struct{} to the object pool
func PutStringMap(m map[string]struct{}) {
	if m == nil {
		return
	}

	for k := range m {
		delete(m, k)
	}

	StringMapPool.Put(m)
}
