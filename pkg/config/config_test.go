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
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/tests/base"
)

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(configSuite))
}

type configSuite struct {
	base.Suite

	fdbDir string
	ckDir  string
}

func (s *configSuite) SetupSuite() {
	s.Suite.SetupSuite()

	s.fdbDir = "/fdb"
	s.ckDir = "/clickhouse"
}

func (s *configSuite) newConfig() *Config {
	cfg := NewConfigWithDefaults()
	cfg.Nodes = []Node{
		{
			Name:     "node1",
			Host:     "localhost",
			Username: "node1",
		},
	}
	cfg.Services.Fdb.Nodes = []string{"node1"}
	cfg.Services.Clickhouse.Nodes = []string{"node1"}
	cfg.Services.Grafana.Nodes = []string{"node1"}
	cfg.Services.Monitor.Nodes = []string{"node1"}
	cfg.Services.Mgmtd.Nodes = []string{"node1"}
	cfg.Services.Meta.Nodes = []string{"node1"}
	cfg.Services.Storage.Nodes = []string{"node1"}
	cfg.Services.Client.Nodes = []string{"node1"}

	return cfg
}

func (s *configSuite) newConfigWithDefaults() *Config {
	cfg := s.newConfig()
	cfg.Services.Fdb.Port = 49990
	cfg.Nodes[0].Port = 123
	cfg.Services.Mgmtd.RDMAListenPort = 8033
	cfg.Services.Mgmtd.TCPListenPort = 9003
	cfg.Services.Meta.RDMAListenPort = 8011
	cfg.Services.Meta.TCPListenPort = 9301
	cfg.Services.Storage.RDMAListenPort = 8092
	cfg.Services.Storage.TCPListenPort = 9072
	cfg.Services.Storage.DiskNumPerNode = 3

	return cfg
}

func (s *configSuite) TestValidateConfig() {
	cfg := s.newConfig()
	cfg.WorkDir = "/root/3fs"
	cfg.Services.Fdb.Port = 49990
	cfg.Services.Mgmtd.RDMAListenPort = 8000
	cfg.Services.Mgmtd.TCPListenPort = 9000
	cfg.Services.Meta.RDMAListenPort = 8701
	cfg.Services.Meta.TCPListenPort = 9091
	cfg.Services.Storage.RDMAListenPort = 8702
	cfg.Services.Storage.TCPListenPort = 9092
	cfg.Services.Storage.DiskNumPerNode = 3
	cfg.Nodes[0].Port = 123
	cfgExp := *cfg

	s.NoError(cfg.SetValidate("/root/3fs", ""))

	s.Equal(&cfgExp, cfg)
}

func (s *configSuite) TestSetConfigDefaults() {
	cfg := s.newConfig()

	s.NoError(cfg.SetValidate("/root", ""))

	s.Equal(cfg.LogLevel, "INFO")
	s.Equal(cfg.Services.Mgmtd.RDMAListenPort, 8000)
	s.Equal(cfg.Services.Mgmtd.TCPListenPort, 9000)
	s.Equal(cfg.Services.Fdb.Port, 4500)
	s.Equal(cfg.Nodes[0].Port, 22)
}

func (s *configSuite) TestSetConfigWithLogLevel() {
	cfg := s.newConfig()
	cfg.LogLevel = "DEBUG"

	s.NoError(cfg.SetValidate("/root", ""))

	s.Equal(cfg.LogLevel, "DEBUG")
	s.Equal(cfg.Services.Mgmtd.RDMAListenPort, 8000)
	s.Equal(cfg.Services.Mgmtd.TCPListenPort, 9000)
	s.Equal(cfg.Services.Fdb.Port, 4500)
	s.Equal(cfg.Nodes[0].Port, 22)
}

func (s *configSuite) TestSetConfigWithRegistry() {
	cfg := s.newConfig()

	s.NoError(cfg.SetValidate("/root", "harbor.open3fs.com"))

	s.Equal(cfg.Images.Registry, "harbor.open3fs.com")
}

func (s *configSuite) TestValidWithNoName() {
	cfg := s.newConfigWithDefaults()
	cfg.Nodes[0].Name = ""

	s.Error(cfg.SetValidate("", ""), "name is required")
}

func (s *configSuite) TestValidWithInvalidNetworkType() {
	cfg := s.newConfigWithDefaults()
	cfg.NetworkType = "invalid"

	s.Error(cfg.SetValidate("", ""), "invalid network type: invalid")
}

func (s *configSuite) TestValidWithNoNodes() {
	cfg := s.newConfigWithDefaults()
	cfg.Nodes = nil

	s.Error(cfg.SetValidate("", ""), "nodes is required")
}

func (s *configSuite) TestValidWithNoNodeName() {
	cfg := s.newConfigWithDefaults()
	cfg.Nodes[0].Name = ""

	s.Error(cfg.SetValidate("", ""), "nodes[0].name is required")
}

func (s *configSuite) TestValidWithDupNodeName() {
	cfg := s.newConfigWithDefaults()
	cfg.Nodes = append(cfg.Nodes, cfg.Nodes[0])

	s.Error(cfg.SetValidate("", ""), "duplicate node name: node1")
}

func (s *configSuite) TestValidWithNoNodeHost() {
	cfg := s.newConfigWithDefaults()
	cfg.Nodes[0].Host = ""

	s.Error(cfg.SetValidate("", ""), "nodes[0].host is required")
}

func (s *configSuite) TestValidWithDupNodeHost() {
	cfg := s.newConfigWithDefaults()
	cfg.Nodes = append(cfg.Nodes, Node{
		Name: "node2",
		Host: "localhost",
		Port: 1234,
	})

	s.Error(cfg.SetValidate("", ""), "duplicate node host: localhost")
}

func (s *configSuite) TestValidWithNoServiceNode() {
	cfg := s.newConfigWithDefaults()
	cfg.Services.Fdb.Nodes = nil

	s.Error(cfg.SetValidate("", ""), "nodes of fdb service is required")
}

func (s *configSuite) TestValidWithServiceNodeNotExists() {
	cfg := s.newConfigWithDefaults()
	cfg.Services.Fdb.Nodes = []string{"node2"}

	s.Error(cfg.SetValidate("", ""), "node node2 of fdb service not exists in node list")
}

func (s *configSuite) TestValidWithNoServiceNodeSup() {
	cfg := s.newConfigWithDefaults()
	cfg.Services.Fdb.Nodes = []string{"node1", "node1"}

	s.Error(cfg.SetValidate("", ""), "duplicate node node1 in fdb service")
}

func (s *configSuite) TestValidWithInvalidStorageDiskType() {
	cfg := s.newConfigWithDefaults()
	cfg.Services.Storage.DiskType = "invalid"

	s.Error(cfg.SetValidate("", ""), "invalid disk type of storage service: invalid")
}

func (s *configSuite) TestValidWithNoClientMountPoint() {
	cfg := s.newConfigWithDefaults()
	cfg.Services.Client.HostMountpoint = ""

	s.Error(cfg.SetValidate("", ""), "services.client.hostMountpoint is required")
}

func (s *configSuite) TestWithImageNoTag() {
	cfg := s.newConfigWithDefaults()
	cfg.Images.Fdb.Tag = ""

	s.Error(cfg.SetValidate("", ""), "images.fdb.tag is required")
}

func (s *configSuite) TestWithImageNoRepo() {
	cfg := s.newConfigWithDefaults()
	cfg.Images.Fdb.Repo = ""

	s.Error(cfg.SetValidate("", ""), "images.fdb.repo is required")
}

func (s *configSuite) TestWithDupGroupName() {
	cfg := s.newConfigWithDefaults()
	cfg.NodeGroups = append(cfg.NodeGroups, NodeGroup{
		Name: "group1",
	})
	cfg.NodeGroups = append(cfg.NodeGroups, NodeGroup{
		Name: "group1",
	})

	s.Error(cfg.SetValidate("", ""), "duplicate node group name: group1")
}

func (s *configSuite) TestWithNoNodeGroupName() {
	cfg := s.newConfigWithDefaults()
	cfg.NodeGroups = append(cfg.NodeGroups, NodeGroup{
		Name: "",
	})

	s.Error(cfg.SetValidate("", ""), "nodeGroups[0].name is required")
}

func (s *configSuite) TestWithNoNodeGroupIPOverlap() {
	cfg := s.newConfigWithDefaults()
	cfg.NodeGroups = append(cfg.NodeGroups, NodeGroup{
		Name:    "gp1",
		IPBegin: "10.1.1.1",
		IPEnd:   "10.1.1.5",
	})
	cfg.NodeGroups = append(cfg.NodeGroups, NodeGroup{
		Name:    "gp2",
		IPBegin: "10.1.1.4",
		IPEnd:   "10.1.1.6",
	})

	s.Error(cfg.SetValidate("", ""), "node group gp2 and gp1 ip range overlap")
}

func (s *configSuite) TestWithEmptyNodeGroupIP() {
	cfg := s.newConfigWithDefaults()
	cfg.NodeGroups = append(cfg.NodeGroups, NodeGroup{
		Name:    "gp1",
		IPBegin: "1.1.1.1",
		IPEnd:   "1.1.1.0",
	})

	s.Error(cfg.SetValidate("", ""), "node group gp1 ip range is empty")
}

func (s *configSuite) TestWithNodeNodeGroupIPDup() {
	cfg := s.newConfigWithDefaults()
	cfg.Nodes = append(cfg.Nodes, Node{
		Name:     "gp1",
		Host:     "1.1.1.2",
		Username: "root",
	})
	cfg.NodeGroups = append(cfg.NodeGroups, NodeGroup{
		Name:     "gp1",
		IPBegin:  "1.1.1.1",
		IPEnd:    "1.1.1.3",
		Username: "username",
	})

	s.Error(cfg.SetValidate("", ""), "node ip 1.1.1.2 duplicate in node group gp1")
}

func (s *configSuite) TestWithNodeNodegroupIpDup() {
	cfg := s.newConfigWithDefaults()
	cfg.Nodes = append(cfg.Nodes, Node{
		Name:     "gp1",
		Host:     "1.1.1.2",
		Username: "root",
	})
	cfg.NodeGroups = append(cfg.NodeGroups, NodeGroup{
		Name:     "gp1",
		IPBegin:  "1.1.1.1",
		IPEnd:    "1.1.1.3",
		Username: "username",
	})

	s.Error(cfg.SetValidate("", ""), "node ip 1.1.1.2 duplicate in node group gp1")
}

func (s *configSuite) TestParseNodeGroup() {
	cfg := s.newConfigWithDefaults()
	cfg.NodeGroups = append(cfg.NodeGroups, NodeGroup{
		Name:     "gp1",
		IPBegin:  "1.1.1.1",
		IPEnd:    "1.1.1.3",
		Username: "root",
	})
	cfg.Services.Fdb.NodeGroups = []string{"gp1"}
	cfg.Services.Clickhouse.NodeGroups = []string{"gp1"}
	cfg.Services.Grafana.NodeGroups = []string{"gp1"}
	cfg.Services.Monitor.NodeGroups = []string{"gp1"}
	cfg.Services.Mgmtd.NodeGroups = []string{"gp1"}
	cfg.Services.Meta.NodeGroups = []string{"gp1"}
	cfg.Services.Storage.NodeGroups = []string{"gp1"}
	cfg.Services.Client.NodeGroups = []string{"gp1"}

	s.NoError(cfg.SetValidate("", ""))

	nodesExp := []Node{
		{
			Name:     "gp1-node(1.1.1.1)",
			Host:     "1.1.1.1",
			Username: "root",
			Port:     22,
		},
		{
			Name:     "gp1-node(1.1.1.2)",
			Host:     "1.1.1.2",
			Username: "root",
			Port:     22,
		},
		{
			Name:     "gp1-node(1.1.1.3)",
			Host:     "1.1.1.3",
			Username: "root",
			Port:     22,
		},
	}
	s.Equal(nodesExp, cfg.NodeGroups[0].Nodes)
	serviceNodes := []string{"node1", "gp1-node(1.1.1.1)", "gp1-node(1.1.1.2)", "gp1-node(1.1.1.3)"}
	s.Equal(serviceNodes, cfg.Services.Fdb.Nodes)
	s.Equal(serviceNodes, cfg.Services.Clickhouse.Nodes)
	s.Equal(serviceNodes, cfg.Services.Grafana.Nodes)
	s.Equal(serviceNodes, cfg.Services.Monitor.Nodes)
	s.Equal(serviceNodes, cfg.Services.Mgmtd.Nodes)
	s.Equal(serviceNodes, cfg.Services.Meta.Nodes)
	s.Equal(serviceNodes, cfg.Services.Storage.Nodes)
	s.Equal(serviceNodes, cfg.Services.Client.Nodes)
	nodesExp = append([]Node{cfg.Nodes[0]}, nodesExp...)
	s.Equal(nodesExp, cfg.Nodes)
}

func (s *configSuite) TestWithInvalidImageArch() {
	cfg := s.newConfigWithDefaults()
	cfg.Images.Arch = "wasm"

	s.Error(cfg.SetValidate("", ""), "unsupported arch of images: wasm, supported archs: amd64, arm64")
}
