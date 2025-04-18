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

package cache

import (
	"fmt"
	"sync"

	"github.com/open3fs/m3fs/pkg/config"
)

// NodeCache provides caching for node-related data
type NodeCache struct {
	manager         *Manager
	serviceCache    sync.Map
	nodeGroupCache  sync.Map
	nodeInListCache sync.Map
}

// NewNodeCache creates a new NodeCache
func NewNodeCache(manager *Manager) *NodeCache {
	return &NodeCache{
		manager: manager,
	}
}

// GetServiceNodes retrieves cached service nodes or populates with getterFunc if not found
func (c *NodeCache) GetServiceNodes(serviceType config.ServiceType, getterFunc func() []string) []string {
	cacheKey := string(serviceType)
	if value, ok := c.serviceCache.Load(cacheKey); ok {
		return value.([]string)
	}

	nodes := getterFunc()
	c.CacheServiceNodes(serviceType, nodes)
	return nodes
}

// CacheServiceNodes caches service nodes
func (c *NodeCache) CacheServiceNodes(serviceType config.ServiceType, nodes []string) {
	cacheKey := string(serviceType)
	c.serviceCache.Store(cacheKey, nodes)
}

// GetNodeGroup retrieves cached node group or populates with getterFunc if not found
func (c *NodeCache) GetNodeGroup(groupName string, getterFunc func() []string) []string {
	cacheKey := fmt.Sprintf("nodegroup:%s", groupName)
	if value, ok := c.nodeGroupCache.Load(cacheKey); ok {
		return value.([]string)
	}

	nodes := getterFunc()
	if len(nodes) > 0 {
		c.nodeGroupCache.Store(cacheKey, nodes)
	}
	return nodes
}

// IsNodeInList checks if a node is in a list with caching
func (c *NodeCache) IsNodeInList(nodeName string, nodeList []string) bool {
	// For small lists, do direct check
	if len(nodeList) <= 10 {
		for _, n := range nodeList {
			if n == nodeName {
				return true
			}
		}
		return false
	}

	nodeSet := make(map[string]struct{}, len(nodeList))
	for _, n := range nodeList {
		nodeSet[n] = struct{}{}
	}

	_, exists := nodeSet[nodeName]
	return exists
}

// ClearCache clears all cached data
func (c *NodeCache) ClearCache() {
	c.serviceCache = sync.Map{}
	c.nodeGroupCache = sync.Map{}
	c.nodeInListCache = sync.Map{}
}
