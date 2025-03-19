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

// Fdb is the fdb config definition
type Fdb struct {
	ContainerName      string `yaml:"containerName"`
	Nodes              []string
	Port               int
	WaitClusterTimeout time.Duration
}

// Clickhouse is the click house config definition
type Clickhouse struct {
	ContainerName string `yaml:"containerName"`
	Nodes         []string
	Db            string `yaml:"db"`
	User          string `yaml:"user"`
	Password      string `yaml:"password"`
	TCPPort       int    `yaml:"tcpPort"`
}

// Monitor is the monitor config definition
type Monitor struct {
	ContainerName string `yaml:"containerName"`
	Nodes         []string
	Port          int `yaml:"port"`
}

// Mgmtd is the 3fs mgmtd service config definition
type Mgmtd struct {
	ContainerName  string `yaml:"containerName"`
	Nodes          []string
	ChunkSize      int `yaml:"chunkSize"`
	StripeSize     int `yaml:"stripeSize"`
	RDMAListenPort int `yaml:"rdmaListenPort,omitempty"`
	TCPListenPort  int `yaml:"tcpListenPort,omitempty"`
}

// Meta is the 3fs meta service config definition
type Meta struct {
	ContainerName  string `yaml:"containerName"`
	Nodes          []string
	RDMAListenPort int `yaml:"rdmaListenPort,omitempty"`
	TCPListenPort  int `yaml:"tcpListenPort,omitempty"`
}

// Storage is the 3fs storage config definition
type Storage struct {
	ContainerName       string `yaml:"containerName"`
	Nodes               []string
	DiskType            DiskType `yaml:"diskType,omitempty"`
	DiskNumPerNode      int      `yaml:"diskNumPerNode,omitempty"`
	RDMAListenPort      int      `yaml:"rdmaListenPort,omitempty"`
	TCPListenPort       int      `yaml:"tcpListenPort,omitempty"`
	ReplicationFactor   int      `yaml:"replicationFactor,omitempty"`
	MinTargetNumPerDisk int      `yaml:"minTargetNumPerDisk,omitempty"`
	TargetNumPerDisk    int      `yaml:"targetNumPerDisk,omitempty"`
	TargetIDPrefix      int      `yaml:"targetIDPrefix,omitempty"`
	ChainIDPrefix       int      `yaml:"chainIDPrefix,omitempty"`
}

// Client is the 3fs client config definition
type Client struct {
	ContainerName  string `yaml:"containerName"`
	Nodes          []string
	HostMountpoint string `yaml:"hostMountpoint"`
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
	Nodes             []Node
	Services          Services       `yaml:"services"`
	Images            Images         `yaml:"images"`
	CmdMaxExitTimeout *time.Duration `yaml:",omitempty"`
}

// SetValidate validates the config and set default values if some fields are missing
func (c *Config) SetValidate(workDir, registry string) error {
	if c.Name == "" {
		return errors.New("name is required")
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
	if len(c.Nodes) == 0 {
		return errors.New("nodes is required")
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
		if node.Port == 0 {
			c.Nodes[i].Port = 22
		}
	}

	validSettings := []struct {
		name    string
		nodes   []string
		require bool
	}{
		{
			"fdb",
			c.Services.Fdb.Nodes,
			true,
		},
		{
			"clickhouse",
			c.Services.Clickhouse.Nodes,
			true,
		},
		{
			"monitor",
			c.Services.Monitor.Nodes,
			true,
		},
		{
			"mgmtd",
			c.Services.Mgmtd.Nodes,
			true,
		},
		{
			"meta",
			c.Services.Mgmtd.Nodes,
			true,
		},
		{
			"storage",
			c.Services.Storage.Nodes,
			true,
		},
		{
			"client",
			c.Services.Client.Nodes,
			false,
		},
	}

	for _, s := range validSettings {
		if err := c.validServiceNodes(s.name, s.nodes, nodeSet, s.require); err != nil {
			return errors.Trace(err)
		}
	}

	if c.Services.Fdb.Port == 0 {
		c.Services.Fdb.Port = 4500
	}

	if c.Services.Mgmtd.RDMAListenPort == 0 {
		c.Services.Mgmtd.RDMAListenPort = 8000
	}
	if c.Services.Mgmtd.TCPListenPort == 0 {
		c.Services.Mgmtd.TCPListenPort = 9000
	}

	if c.Services.Meta.RDMAListenPort == 0 {
		c.Services.Meta.RDMAListenPort = 8001
	}
	if c.Services.Meta.TCPListenPort == 0 {
		c.Services.Meta.TCPListenPort = 9001
	}
	if c.Services.Mgmtd.ChunkSize == 0 {
		c.Services.Mgmtd.ChunkSize = 1048576
	}
	if c.Services.Mgmtd.StripeSize == 0 {
		c.Services.Mgmtd.StripeSize = 16
	}

	if !diskTypes.Contains(c.Services.Storage.DiskType) {
		return errors.Errorf("invalid disk type of storage service: %s", c.Services.Storage.DiskType)
	}
	if c.Services.Storage.RDMAListenPort == 0 {
		c.Services.Storage.RDMAListenPort = 8002
	}
	if c.Services.Storage.TCPListenPort == 0 {
		c.Services.Storage.TCPListenPort = 9002
	}
	if c.Services.Storage.DiskNumPerNode == 0 {
		c.Services.Storage.DiskNumPerNode = 1
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
	service string, nodes []string, nodeSet *utils.Set[string], required bool) error {

	if required && len(nodes) == 0 {
		return errors.Errorf("nodes of %s service is required", service)
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
				ContainerName: "3fs-meta",
			},
			Storage: Storage{
				ContainerName:       "3fs-storage",
				DiskType:            DiskTypeNvme,
				ReplicationFactor:   2,
				DiskNumPerNode:      1,
				MinTargetNumPerDisk: 32,
				TargetNumPerDisk:    32,
				TargetIDPrefix:      1,
				ChainIDPrefix:       9,
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
				Tag:  "20250315",
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
