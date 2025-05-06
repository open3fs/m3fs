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
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/network"
	"github.com/open3fs/m3fs/pkg/utils"
)

// GetServiceNodesFunc is a function that returns the nodes and node groups for a service type
type GetServiceNodesFunc func(cfg *config.Config) ([]string, []string)

// serviceConfigFuncMap is a map of service types to their configuration functions
var serviceConfigFuncMap = map[config.ServiceType]GetServiceNodesFunc{
	config.ServiceStorage: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Storage.Nodes, cfg.Services.Storage.NodeGroups
	},
	config.ServiceFdb: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Fdb.Nodes, cfg.Services.Fdb.NodeGroups
	},
	config.ServiceMeta: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Meta.Nodes, cfg.Services.Meta.NodeGroups
	},
	config.ServiceMgmtd: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Mgmtd.Nodes, cfg.Services.Mgmtd.NodeGroups
	},
	config.ServiceMonitor: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Monitor.Nodes, cfg.Services.Monitor.NodeGroups
	},
	config.ServiceClickhouse: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Clickhouse.Nodes, cfg.Services.Clickhouse.NodeGroups
	},
	config.ServiceClient: func(cfg *config.Config) ([]string, []string) {
		return cfg.Services.Client.Nodes, cfg.Services.Client.NodeGroups
	},
}

// ArchDiagram generates architecture diagrams for m3fs clusters
type ArchDiagram struct {
	cfg     *config.Config
	noColor bool

	// groupMap is a map of node group name to sorted nodes
	groupMap map[string][]string
	// serviceMap is a map of service type to nodes
	serviceMap map[config.ServiceType][]string
}

// NewArchDiagram creates a new ArchDiagram.
func NewArchDiagram(cfg *config.Config, noColor bool) (*ArchDiagram, error) {
	if cfg == nil {
		return nil, errors.Errorf("config is nil")
	}

	diagram := &ArchDiagram{
		cfg:     cfg,
		noColor: noColor,
	}

	groupMap, err := diagram.buildNodeGroupMap()
	if err != nil {
		return nil, errors.Annotatef(err, "build node group map")
	}
	diagram.groupMap = groupMap
	diagram.serviceMap = diagram.buildServiceMap()

	return diagram, nil
}

func (g *ArchDiagram) buildNodeGroupMap() (map[string][]string, error) {
	groupMap := make(map[string][]string)

	for _, nodeGroup := range g.cfg.NodeGroups {
		nodeMap := make(map[string]struct{})
		ipList, err := g.expandNodeGroup(&nodeGroup)
		if err != nil {
			return nil, errors.Annotatef(err, "expand node group %s", nodeGroup.Name)
		}
		for _, ip := range ipList {
			nodeMap[ip] = struct{}{}
		}
		nodes := make([]string, 0, len(nodeMap))
		for ip := range nodeMap {
			nodes = append(nodes, ip)
		}
		groupMap[nodeGroup.Name] = g.sortNodes(nodes)
	}

	return groupMap, nil
}

// getServiceNodeConfig returns nodes and node groups for a service type
func (g *ArchDiagram) getServiceNodeConfig(serviceType config.ServiceType) ([]string, []string) {
	configFunc, ok := serviceConfigFuncMap[serviceType]
	if !ok {
		return nil, nil
	}

	return configFunc(g.cfg)
}

func (g *ArchDiagram) buildServiceMap() map[config.ServiceType][]string {
	serviceMap := make(map[config.ServiceType][]string)

	for _, serviceType := range config.AllServiceTypes {
		nodes, nodeGroups := g.getServiceNodeConfig(serviceType)
		serviceMap[serviceType] = nodes
		for _, nodeGroup := range nodeGroups {
			serviceMap[serviceType] = append(serviceMap[serviceType], g.groupMap[nodeGroup]...)
		}
	}

	return serviceMap
}

func (g *ArchDiagram) getTotalNodeCount() int {
	nodeMap := make(map[string]struct{})

	for _, node := range g.cfg.Nodes {
		if node.Host != "" && net.ParseIP(node.Host) != nil {
			nodeMap[node.Host] = struct{}{}
		}
	}
	for _, nodes := range g.groupMap {
		for _, node := range nodes {
			nodeMap[node] = struct{}{}
		}
	}
	return len(nodeMap)
}

func (g *ArchDiagram) getServiceNodeCounts() map[config.ServiceType]int {
	result := make(map[config.ServiceType]int)
	for _, serviceType := range config.AllServiceTypes {
		result[serviceType] = len(g.serviceMap[serviceType])
	}
	return result
}

func (g *ArchDiagram) sortNodes(nodes []string) []string {
	ipList := make([]netip.Addr, 0, len(nodes))
	for _, n := range nodes {
		ipList = append(ipList, netip.MustParseAddr(n))
	}
	sort.Slice(ipList, func(i, j int) bool { return ipList[i].Less(ipList[j]) })

	sorted := make([]string, 0, len(ipList))
	for _, ip := range ipList {
		sorted = append(sorted, ip.String())
	}
	return sorted
}

func (g *ArchDiagram) getStorageNodes() []string {
	nodeMap := make(map[string]struct{})
	for _, serviceType := range config.AllServiceTypes {
		if serviceType == config.ServiceClient {
			continue
		}
		for _, n := range g.serviceMap[serviceType] {
			nodeMap[n] = struct{}{}
		}
	}

	nodes := []string{}
	for n := range nodeMap {
		nodes = append(nodes, n)
	}
	return nodes
}

// getStorageServices returns the services running on a node
func (g *ArchDiagram) getStorageServices(node string) []string {
	services := make([]string, 0, len(config.AllServiceTypes))

	for _, svcType := range config.AllServiceTypes {
		if svcType == config.ServiceClient {
			continue
		}

		nodes := g.serviceMap[svcType]
		for _, n := range nodes {
			if node == n {
				displayName := config.ServiceDisplayNames[svcType]
				services = append(services, fmt.Sprintf("[%s]", displayName))
			}
		}
	}
	return services
}

// expandNodeGroup expands a node group into individual nodes
func (g *ArchDiagram) expandNodeGroup(nodeGroup *config.NodeGroup) ([]string, error) {
	if len(nodeGroup.Nodes) > 0 {
		ipList := make([]string, 0, len(nodeGroup.Nodes))
		for _, node := range nodeGroup.Nodes {
			if node.Host != "" && net.ParseIP(node.Host) != nil {
				ipList = append(ipList, node.Host)
			}
		}
		if len(ipList) > 0 {
			return ipList, nil
		}
	}

	ipList, err := utils.GenerateIPRange(nodeGroup.IPBegin, nodeGroup.IPEnd)
	if err != nil {
		return nil, errors.Annotatef(err, "parse IP range %s-%s", nodeGroup.IPBegin, nodeGroup.IPEnd)
	}

	return ipList, nil
}

// Render renders the architecture diagram
func (g *ArchDiagram) Render() string {
	b := &strings.Builder{}

	render := NewDiagramRenderer(g.cfg)
	render.ColorEnabled = !g.noColor

	clientNodes := g.serviceMap[config.ServiceClient]
	storageNodes := g.getStorageNodes()
	serviceCounts := g.getServiceNodeCounts()
	totalNodeCount := g.getTotalNodeCount()

	clientDisplayCount := render.CalculateMaxNodeCount(len(clientNodes))
	storageDisplayCount := render.CalculateMaxNodeCount(len(storageNodes))

	clientSectionWidth := render.CalculateNodeRowWidth(clientDisplayCount) - 2
	render.RenderHeader(b, clientSectionWidth)

	if len(clientNodes) > 0 {
		render.RenderClientSection(b, clientNodes)
	}

	if len(clientNodes) > 0 && len(storageNodes) > 0 {
		networkType := string(g.cfg.NetworkType)
		localRunnerCfg := &external.LocalRunnerCfg{
			Logger:         log.Logger,
			MaxExitTimeout: g.cfg.CmdMaxExitTimeout,
		}
		localRunner := external.NewLocalRunner(localRunnerCfg)
		speed := network.GetSpeed(context.TODO(), localRunner, g.cfg.NetworkType)
		render.RenderNetworkConnection(b, networkType, speed, clientDisplayCount, storageDisplayCount)
	}

	if len(storageNodes) > 0 {
		render.RenderStorageSection(b, storageNodes, g.getStorageServices)
	}

	render.RenderSummarySection(b, serviceCounts, totalNodeCount)

	return b.String()
}
