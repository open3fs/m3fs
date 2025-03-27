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

package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/utils"
)

// NetworkType is the type of network definition
type NetworkType string

// defines network types
const (
	NetworkTypeRDMA  NetworkType = "RDMA"
	NetworkTypeRXE   NetworkType = "RXE"
	NetworkTypeERDMA NetworkType = "ERDMA"
)

var networkTypes = utils.NewSet(NetworkTypeRDMA, NetworkTypeRXE, NetworkTypeERDMA)

// DiskType is the type of disk definition
type DiskType string

// defines disk types
const (
	DiskTypeDirectory DiskType = "dir"
	DiskTypeNvme      DiskType = "nvme"
)

var diskTypes = utils.NewSet(DiskTypeDirectory, DiskTypeNvme)

// Node is the node config definition
type Node struct {
	Name          string
	Host          string
	Port          int
	Username      string
	Password      *string  `yaml:",omitempty"`
	RDMAAddresses []string `yaml:"rdmaAddresses,omitempty"`
}

// NodeGroup is the node group config definition
type NodeGroup struct {
	Name     string
	Port     int
	Username string
	Password *string `yaml:",omitempty"`
	IPBegin  string  `yaml:"ipBegin"`
	IPEnd    string  `yaml:"ipEnd"`
	Nodes    []Node  `yaml:"-"`
}

// Fdb is the fdb config definition
type Fdb struct {
	ContainerName      string `yaml:"containerName"`
	Nodes              []string
	NodeGroups         []string `yaml:"nodeGroups"`
	Port               int
	WaitClusterTimeout time.Duration
}

// Clickhouse is the click house config definition
type Clickhouse struct {
	ContainerName string `yaml:"containerName"`
	Nodes         []string
	NodeGroups    []string `yaml:"nodeGroups"`
	Db            string   `yaml:"db"`
	User          string   `yaml:"user"`
	Password      string   `yaml:"password"`
	TCPPort       int      `yaml:"tcpPort"`
}

// Monitor is the monitor config definition
type Monitor struct {
	ContainerName string `yaml:"containerName"`
	Nodes         []string
	NodeGroups    []string `yaml:"nodeGroups"`
	Port          int      `yaml:"port"`
}

// Mgmtd is the 3fs mgmtd service config definition
type Mgmtd struct {
	ContainerName  string `yaml:"containerName"`
	Nodes          []string
	NodeGroups     []string `yaml:"nodeGroups"`
	ChunkSize      int      `yaml:"chunkSize"`
	StripeSize     int      `yaml:"stripeSize"`
	RDMAListenPort int      `yaml:"rdmaListenPort,omitempty"`
	TCPListenPort  int      `yaml:"tcpListenPort,omitempty"`
}

// Meta is the 3fs meta service config definition
type Meta struct {
	ContainerName  string `yaml:"containerName"`
	Nodes          []string
	NodeGroups     []string `yaml:"nodeGroups"`
	RDMAListenPort int      `yaml:"rdmaListenPort,omitempty"`
	TCPListenPort  int      `yaml:"tcpListenPort,omitempty"`
}

// Storage is the 3fs storage config definition
type Storage struct {
	ContainerName     string `yaml:"containerName"`
	Nodes             []string
	NodeGroups        []string `yaml:"nodeGroups"`
	DiskType          DiskType `yaml:"diskType,omitempty"`
	SectorSize        int      `yaml:"sectorSize,omitempty"`
	DiskNumPerNode    int      `yaml:"diskNumPerNode,omitempty"`
	RDMAListenPort    int      `yaml:"rdmaListenPort,omitempty"`
	TCPListenPort     int      `yaml:"tcpListenPort,omitempty"`
	ReplicationFactor int      `yaml:"replicationFactor,omitempty"`
	TargetNumPerDisk  int      `yaml:"targetNumPerDisk,omitempty"`
	TargetIDPrefix    int      `yaml:"targetIDPrefix,omitempty"`
	ChainIDPrefix     int      `yaml:"chainIDPrefix,omitempty"`
}

// Client is the 3fs client config definition
type Client struct {
	ContainerName  string `yaml:"containerName"`
	Nodes          []string
	NodeGroups     []string `yaml:"nodeGroups"`
	HostMountpoint string   `yaml:"hostMountpoint"`
}

// Services is the services config definition
type Services struct {
	Fdb        Fdb
	Clickhouse Clickhouse
	Monitor    Monitor
	Mgmtd      Mgmtd
	Meta       Meta
	Storage    Storage
	Client     Client
}

// Config is the 3fs cluster config definition
type Config struct {
	Name              string
	WorkDir           string      `yaml:"workDir"`
	NetworkType       NetworkType `yaml:"networkType"`
	LogLevel          string      `yaml:"logLevel"`
	Nodes             []Node
	NodeGroups        []NodeGroup    `yaml:"nodeGroups"`
	Services          Services       `yaml:"services"`
	Images            Images         `yaml:"images"`
	CmdMaxExitTimeout *time.Duration `yaml:",omitempty"`
}

func (c *Config) parseValidateNodeGroups(hostSet *utils.Set[string]) (map[string]*NodeGroup, error) {
	nodeGroups := make(map[string]*NodeGroup, len(c.NodeGroups))
	for i := range c.NodeGroups {
		nodeGroup := &c.NodeGroups[i]
		if _, ok := nodeGroups[nodeGroup.Name]; ok {
			return nil, errors.Errorf("duplicate node group name: %s", nodeGroup.Name)
		}
		if nodeGroup.Name == "" {
			return nil, errors.Errorf("nodeGroup[%d].name is required", i)
		}
		if nodeGroup.Username == "" {
			return nil, errors.Errorf("nodeGroup[%d].username is required", i)
		}
		for _, existNodeGroup := range nodeGroups {
			// check range overlap
			if (nodeGroup.IPBegin <= existNodeGroup.IPBegin && nodeGroup.IPEnd >= existNodeGroup.IPBegin) ||
				(nodeGroup.IPBegin >= existNodeGroup.IPBegin && nodeGroup.IPBegin <= existNodeGroup.IPEnd) {
				return nil, errors.Errorf("node group %s and %s ip range overlap",
					nodeGroup.Name, existNodeGroup.Name)
			}
		}
		nodeGroups[nodeGroup.Name] = nodeGroup
		nodeGroupIPs, err := utils.GenerateIPRange(nodeGroup.IPBegin, nodeGroup.IPEnd)
		if err != nil {
			return nil, errors.Annotatef(err, "generate ip range for node group %s", nodeGroup.Name)
		}
		if len(nodeGroupIPs) == 0 {
			return nil, errors.Errorf("node group %s ip range is empty", nodeGroup.Name)
		}
		c.NodeGroups[i].Nodes = make([]Node, len(nodeGroupIPs))
		if nodeGroup.Port == 0 {
			nodeGroup.Port = 22
		}
		for j, nodeGroupIP := range nodeGroupIPs {
			if hostSet.Contains(nodeGroupIP) {
				return nil, errors.Errorf("node ip %s duplicate with node group %s",
					nodeGroupIP, nodeGroup.Name)
			}
			nodeGroup.Nodes[j] = Node{
				Name:     fmt.Sprintf("%s-node(%s)", nodeGroup.Name, nodeGroupIP),
				Host:     nodeGroupIP,
				Port:     nodeGroup.Port,
				Username: nodeGroup.Username,
				Password: nodeGroup.Password,
			}
		}
	}

	return nodeGroups, nil
}

func (c *Config) parseNodeGroupToNodes(nodeGroupMap map[string]*NodeGroup) {
	for _, nodeGroup := range nodeGroupMap {
		c.Nodes = append(c.Nodes, nodeGroup.Nodes...)
	}

	for _, nodeGroupName := range c.Services.Fdb.NodeGroups {
		nodeGroup := nodeGroupMap[nodeGroupName]
		for _, node := range nodeGroup.Nodes {
			c.Services.Fdb.Nodes = append(c.Services.Fdb.Nodes, node.Name)
		}
	}
	for _, nodeGroupName := range c.Services.Clickhouse.NodeGroups {
		nodeGroup := nodeGroupMap[nodeGroupName]
		for _, node := range nodeGroup.Nodes {
			c.Services.Clickhouse.Nodes = append(c.Services.Clickhouse.Nodes, node.Name)
		}
	}
	for _, nodeGroupName := range c.Services.Monitor.NodeGroups {
		nodeGroup := nodeGroupMap[nodeGroupName]
		for _, node := range nodeGroup.Nodes {
			c.Services.Monitor.Nodes = append(c.Services.Monitor.Nodes, node.Name)
		}
	}
	for _, nodeGroupName := range c.Services.Mgmtd.NodeGroups {
		nodeGroup := nodeGroupMap[nodeGroupName]
		for _, node := range nodeGroup.Nodes {
			c.Services.Mgmtd.Nodes = append(c.Services.Mgmtd.Nodes, node.Name)
		}
	}
	for _, nodeGroupName := range c.Services.Meta.NodeGroups {
		nodeGroup := nodeGroupMap[nodeGroupName]
		for _, node := range nodeGroup.Nodes {
			c.Services.Meta.Nodes = append(c.Services.Meta.Nodes, node.Name)
		}
	}
	for _, nodeGroupName := range c.Services.Storage.NodeGroups {
		nodeGroup := nodeGroupMap[nodeGroupName]
		for _, node := range nodeGroup.Nodes {
			c.Services.Storage.Nodes = append(c.Services.Storage.Nodes, node.Name)
		}
	}
	for _, nodeGroupName := range c.Services.Client.NodeGroups {
		nodeGroup := nodeGroupMap[nodeGroupName]
		for _, node := range nodeGroup.Nodes {
			c.Services.Client.Nodes = append(c.Services.Client.Nodes, node.Name)
		}
	}
}

// SetValidate validates the config and set default values if some fields are missing
func (c *Config) SetValidate(workDir, registry string) error {
	if c.Name == "" {
		return errors.New("name is required")
	}
	if c.LogLevel == "" {
		c.LogLevel = "INFO"
	}
	if workDir != "" {
		c.WorkDir = workDir
	} else if c.WorkDir == "" {
		var err error
		c.WorkDir, err = os.Getwd()
		if err != nil {
			return errors.Trace(err)
		}
	}
	if registry != "" {
		c.Images.Registry = registry
	}
	upperNetwork := NetworkType(strings.ToUpper(string(c.NetworkType)))
	if !networkTypes.Contains(upperNetwork) {
		return errors.Errorf("invalid network type: %s", c.NetworkType)
	}
	c.NetworkType = upperNetwork
	if len(c.Nodes) == 0 && len(c.NodeGroups) == 0 {
		return errors.New("nodes or nodeGroups is required")
	}
	nodeSet := utils.NewSet[string]()
	nodeHostSet := utils.NewSet[string]()
	for i, node := range c.Nodes {
		if node.Name == "" {
			return errors.Errorf("nodes[%d].name is required", i)
		}
		if !nodeSet.AddIfNotExists(node.Name) {
			return errors.Errorf("duplicate node name: %s", node.Name)
		}
		if node.Host == "" {
			return errors.Errorf("nodes[%d].host is required", i)
		}
		if !nodeHostSet.AddIfNotExists(node.Host) {
			return errors.Errorf("duplicate node host: %s", node.Host)
		}
		if node.Username == "" {
			return errors.Errorf("nodes[%d].username is required", i)
		}
		if node.Port == 0 {
			c.Nodes[i].Port = 22
		}
	}

	nodeGroupMap, err := c.parseValidateNodeGroups(nodeHostSet)
	if err != nil {
		return errors.Trace(err)
	}

	validSettings := []struct {
		name       string
		nodes      []string
		nodeGroups []string
		require    bool
	}{
		{
			"fdb",
			c.Services.Fdb.Nodes,
			c.Services.Fdb.NodeGroups,
			true,
		},
		{
			"clickhouse",
			c.Services.Clickhouse.Nodes,
			c.Services.Clickhouse.NodeGroups,
			true,
		},
		{
			"monitor",
			c.Services.Monitor.Nodes,
			c.Services.Monitor.NodeGroups,
			true,
		},
		{
			"mgmtd",
			c.Services.Mgmtd.Nodes,
			c.Services.Mgmtd.NodeGroups,
			true,
		},
		{
			"meta",
			c.Services.Meta.Nodes,
			c.Services.Meta.NodeGroups,
			true,
		},
		{
			"storage",
			c.Services.Storage.Nodes,
			c.Services.Storage.NodeGroups,
			true,
		},
		{
			"client",
			c.Services.Client.Nodes,
			c.Services.Client.NodeGroups,
			false,
		},
	}

	for _, s := range validSettings {
		if err := c.validServiceNodes(s.name, s.nodes, s.nodeGroups, nodeSet,
			nodeGroupMap, s.require); err != nil {

			return errors.Trace(err)
		}
	}

	c.parseNodeGroupToNodes(nodeGroupMap)

	if !diskTypes.Contains(c.Services.Storage.DiskType) {
		return errors.Errorf("invalid disk type of storage service: %s", c.Services.Storage.DiskType)
	}
	if c.Services.Client.HostMountpoint == "" {
		return errors.New("services.client.hostMountpoint is required")
	}

	if err := c.validImages(); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (c *Config) validServiceNodes(
	service string, nodes []string, nodeGroups []string, nodeSet *utils.Set[string],
	nodeGroupMap map[string]*NodeGroup, required bool) error {

	if required && len(nodes) == 0 && len(nodeGroups) == 0 {
		return errors.Errorf("nodes or nodeGroups of %s service is required", service)
	}
	serviceNodeSet := utils.NewSet[string]()
	for _, node := range nodes {
		if !nodeSet.Contains(node) {
			return errors.Errorf("node %s of  %s service not exists in node list", node, service)
		}
		if !serviceNodeSet.AddIfNotExists(node) {
			return errors.Errorf("duplicate node %s in %s service", node, service)
		}
	}

	serviceNodeGroupSet := utils.NewSet[string]()
	for _, nodeGroup := range nodeGroups {
		if _, ok := nodeGroupMap[nodeGroup]; !ok {
			return errors.Errorf("node group %s of %s service not exists in node group list",
				nodeGroup, service)
		}
		if !serviceNodeGroupSet.AddIfNotExists(nodeGroup) {
			return errors.Errorf("duplicate node group %s in %s service", nodeGroup, service)
		}
		for _, node := range nodeGroupMap[nodeGroup].Nodes {
			nodes = append(nodes, node.Name)
		}
	}

	return nil
}

func (c *Config) validImages() error {
	imgs := []struct {
		imgName string
		image   Image
	}{
		{
			imgName: "fdb",
			image:   c.Images.Fdb,
		},
		{
			imgName: "clickhouse",
			image:   c.Images.Clickhouse,
		},
		{
			imgName: "3fs",
			image:   c.Images.FFFS,
		},
	}
	for _, img := range imgs {
		if img.image.Tag == "" {
			return errors.Errorf("images.%s.tag is required", img.imgName)
		}
		if img.image.Repo == "" {
			return errors.Errorf("images.%s.repo is required", img.imgName)
		}
	}

	return nil
}

// NewConfigWithDefaults creates a new config with default values
func NewConfigWithDefaults() *Config {
	return &Config{
		Name:        "3fs",
		NetworkType: NetworkTypeRDMA,
		LogLevel:    "INFO",
		Services: Services{
			Fdb: Fdb{
				ContainerName:      "3fs-fdb",
				Port:               4500,
				WaitClusterTimeout: 120 * time.Second,
			},
			Clickhouse: Clickhouse{
				ContainerName: "3fs-clickhouse",
				Db:            "3fs",
				User:          "default",
				Password:      "password",
				TCPPort:       8999,
			},
			Monitor: Monitor{
				ContainerName: "3fs-monitor",
				Port:          10000,
			},
			Mgmtd: Mgmtd{
				ContainerName:  "3fs-mgmtd",
				ChunkSize:      1048576,
				StripeSize:     16,
				RDMAListenPort: 8000,
				TCPListenPort:  9000,
			},
			Meta: Meta{
				ContainerName:  "3fs-meta",
				RDMAListenPort: 8001,
				TCPListenPort:  9001,
			},
			Storage: Storage{
				ContainerName:     "3fs-storage",
				DiskType:          DiskTypeNvme,
				SectorSize:        4096,
				RDMAListenPort:    8002,
				TCPListenPort:     9002,
				ReplicationFactor: 2,
				DiskNumPerNode:    1,
				TargetNumPerDisk:  32,
				TargetIDPrefix:    1,
				ChainIDPrefix:     9,
			},
			Client: Client{
				ContainerName:  "3fs-client",
				HostMountpoint: "/mnt/3fs",
			},
		},
		Images: Images{
			Registry: "",
			FFFS: Image{
				Repo: "open3fs/3fs",
				Tag:  "20250327",
			},
			Fdb: Image{
				Repo: "open3fs/foundationdb",
				Tag:  "7.3.63",
			},
			Clickhouse: Image{
				Repo: "open3fs/clickhouse",
				Tag:  "25.1-jammy",
			},
		},
	}
}
