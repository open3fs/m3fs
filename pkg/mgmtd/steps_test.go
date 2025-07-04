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

package mgmtd

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
	ttask "github.com/open3fs/m3fs/tests/task"
)

var suiteRun = suite.Run

func TestInitClusterStepSuite(t *testing.T) {
	suiteRun(t, &initClusterStepSuite{})
}

type initClusterStepSuite struct {
	ttask.StepSuite

	step      *initClusterStep
	configDir string
	logDir    string
}

func (s *initClusterStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &initClusterStep{}
	s.configDir = "/root/3fs/mgmtd/config.d"
	s.logDir = "/root/3fs/mgmtd/log"
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
	s.Runtime.Store(task.RuntimeFdbClusterFileContentKey, "xxxx")
}

func (s *initClusterStepSuite) TestInitCluster() {
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageName3FS)
	s.NoError(err)
	s.MockFS.On("MkdirAll", s.logDir).Return(nil)
	s.MockDocker.On("Run", &external.RunArgs{
		Image:      img,
		Name:       &s.Cfg.Services.Mgmtd.ContainerName,
		Entrypoint: common.Pointer("''"),
		Rm:         common.Pointer(true),
		Command: []string{
			"/opt/3fs/bin/admin_cli",
			"-cfg", "/opt/3fs/etc/admin_cli.toml",
			"'init-cluster --mgmtd /opt/3fs/etc/mgmtd_main.toml 1 1048576 16'",
		},
		HostNetwork: true,
		Volumes: []*external.VolumeArgs{
			{
				Source: "/dev",
				Target: "/dev",
			},
			{
				Source: s.configDir,
				Target: "/opt/3fs/etc",
			},
			{
				Source: s.logDir,
				Target: "/var/log/3fs",
			},
		},
	}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockFS.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func TestGenAdminCliShellSuite(t *testing.T) {
	suiteRun(t, &genAdminCliShellStepSuite{})
}

type genAdminCliShellStepSuite struct {
	ttask.StepSuite

	step *genAdminCliShellStep
}

func (s *genAdminCliShellStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &genAdminCliShellStep{}
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *genAdminCliShellStepSuite) Test() {
	s.MockLocalFS.On("MkdirTemp", os.TempDir(), "3fs-mgmtd").
		Return("/tmp/3fs-mgmtd.xxx", nil)
	shellContent := `#!/bin/bash

docker exec -it 3fs-mgmtd /opt/3fs/bin/admin_cli -cfg /opt/3fs/etc/admin_cli.toml -- $@
`
	s.MockLocalFS.On("WriteFile", "/tmp/3fs-mgmtd.xxx/admin_cli.sh",
		[]byte(shellContent), os.FileMode(0777)).Return(nil)
	s.MockRunner.On("Scp", "/tmp/3fs-mgmtd.xxx/admin_cli.sh", "/root/3fs/admin_cli.sh").Return(nil)
	s.MockLocalFS.On("RemoveAll", "/tmp/3fs-mgmtd.xxx").Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockLocalFS.AssertExpectations(s.T())
	s.MockRunner.AssertExpectations(s.T())
}

func TestInitUserAndChainSuite(t *testing.T) {
	suiteRun(t, &initUserAndChainStepSuite{})
}

type initUserAndChainStepSuite struct {
	ttask.StepSuite

	step *initUserAndChainStep
}

func (s *initUserAndChainStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = &initUserAndChainStep{}
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
	s.Runtime.Store(task.RuntimeMgmtdServerAddressesKey, `["RDMA://10.16.28.58:8000"]`)
}

//nolint:lll
func (s *initUserAndChainStepSuite) Test() {
	containerName := s.Runtime.Services.Mgmtd.ContainerName
	s.MockDocker.On("Exec", containerName, "/opt/3fs/bin/admin_cli", []string{
		"-cfg", "/opt/3fs/etc/admin_cli.toml",
		"--config.mgmtd_client.mgmtd_server_addresses", `'["RDMA://10.16.28.58:8000"]'`,
		`"user-add --root --admin 0 root"`,
	}).Return(`Uid                0
Name               root
Token              AAA8WCoB8QAt8bFw2wBupzjA(Expired at N/A)
IsRootUser         true
IsAdmin            true
Gid                0
SupplementaryGids`, nil)
	s.MockDocker.On("Exec", containerName, "bash", []string{
		"-c",
		`"echo AAA8WCoB8QAt8bFw2wBupzjA > /opt/3fs/etc/token.txt"`,
	}).Return("", nil)

	s.Runtime.Services.Storage.Nodes = []string{"test-3fs-1", "test-3fs-2"}
	s.Runtime.Cfg.CheckStatusInterval = 10 * time.Millisecond
	s.MockDocker.On("Exec", containerName, "/opt/3fs/bin/admin_cli", []string{
		"--cfg", "/opt/3fs/etc/admin_cli.toml",
		"--config.mgmtd_client.mgmtd_server_addresses", `'["RDMA://10.16.28.58:8000"]'`,
		"--config.user_info.token", "AAA8WCoB8QAt8bFw2wBupzjA",
		`"list-nodes"`,
	}).Return(`Id     Type     Status               Hostname    Pid  Tags  LastHeartbeatTime    ConfigVersion  ReleaseVersion
1      MGMTD    PRIMARY_MGMTD        test-3fs-1  1    []    N/A                  1(UPTODATE)    250228-dev-1-999999-ee9a5cee
100    META     HEARTBEAT_CONNECTED  test-3fs-1  1    []    2025-06-30 04:22:52  1(UPTODATE)    250228-dev-1-999999-ee9a5cee`, nil).Once()
	s.MockDocker.On("Exec", containerName, "/opt/3fs/bin/admin_cli", []string{
		"--cfg", "/opt/3fs/etc/admin_cli.toml",
		"--config.mgmtd_client.mgmtd_server_addresses", `'["RDMA://10.16.28.58:8000"]'`,
		"--config.user_info.token", "AAA8WCoB8QAt8bFw2wBupzjA",
		`"list-nodes"`,
	}).Return(`Id     Type     Status               Hostname    Pid  Tags  LastHeartbeatTime    ConfigVersion  ReleaseVersion
1      MGMTD    PRIMARY_MGMTD        test-3fs-1  1    []    N/A                  1(UPTODATE)    250228-dev-1-999999-ee9a5cee
100    META     HEARTBEAT_CONNECTED  test-3fs-1  1    []    2025-06-30 04:22:52  1(UPTODATE)    250228-dev-1-999999-ee9a5cee
10001  STORAGE  HEARTBEAT_CONNECTED  test-3fs-1  1    []    2025-06-30 04:22:50  1(UPTODATE)    250228-dev-1-999999-ee9a5cee
10002  STORAGE  HEARTBEAT_CONNECTED  test-3fs-2  1    []    2025-06-30 04:22:52  1(UPTODATE)    250228-dev-1-999999-ee9a5cee`, nil)

	s.MockDocker.On("Exec", containerName, "python3", []string{
		"/opt/3fs/data_placement/src/model/data_placement.py",
		"-ql", "-relax", "-type", "CR",
		"--num_nodes", strconv.Itoa(len(s.Runtime.Services.Storage.Nodes)),
		"--replication_factor", strconv.Itoa(s.Runtime.Services.Storage.ReplicationFactor),
		"--min_targets_per_disk", strconv.Itoa(s.Runtime.Services.Storage.TargetNumPerDisk),
	}).Return(`2025-03-15 08:34:31.409 | INFO     | __main__:check_solution:332`+
		`- total_traffic=64.0 max_total_traffic=64
		2025-03-15 08:34:31.640 | SUCCESS  | __main__:run:148 - `+
		`saved solution to: output/DataPlacementModel-v_2-b_32-r_32-k_2-λ_32-lb_1-ub_0`, nil)
	s.MockDocker.On("Exec", containerName, "python3", []string{
		"/opt/3fs/data_placement/src/setup/gen_chain_table.py",
		"--chain_table_type", "CR",
		"--node_id_begin", "10001",
		"--node_id_end", strconv.Itoa(10000 + len(s.Runtime.Services.Storage.Nodes)),
		"--num_disks_per_node", strconv.Itoa(s.Runtime.Services.Storage.DiskNumPerNode),
		"--num_targets_per_disk", strconv.Itoa(s.Runtime.Services.Storage.TargetNumPerDisk),
		"--target_id_prefix", strconv.FormatInt(s.Runtime.Services.Storage.TargetIDPrefix, 10),
		"--chain_id_prefix", strconv.FormatInt(s.Runtime.Services.Storage.ChainIDPrefix, 10),
		"--incidence_matrix_path", "output/DataPlacementModel-v_2-b_32-r_32-k_2-λ_32-lb_1-ub_0/incidence_matrix.pickle",
	}).Return("", nil)
	s.MockDocker.On("Exec", containerName, "bash", []string{
		"-c",
		`"/opt/3fs/bin/admin_cli --cfg /opt/3fs/etc/admin_cli.toml ` +
			`--config.mgmtd_client.mgmtd_server_addresses '[\"RDMA://10.16.28.58:8000\"]' ` +
			`--config.user_info.token AAA8WCoB8QAt8bFw2wBupzjA < output/create_target_cmd.txt"`,
	}).Return("", nil)
	s.MockDocker.On("Exec", containerName, "/opt/3fs/bin/admin_cli", []string{
		"--cfg", "/opt/3fs/etc/admin_cli.toml",
		"--config.mgmtd_client.mgmtd_server_addresses", `'["RDMA://10.16.28.58:8000"]'`,
		"--config.user_info.token", "AAA8WCoB8QAt8bFw2wBupzjA",
		`"upload-chains output/generated_chains.csv"`,
	}).Return("", nil)
	s.MockDocker.On("Exec", containerName, "/opt/3fs/bin/admin_cli", []string{
		"--cfg", "/opt/3fs/etc/admin_cli.toml",
		"--config.mgmtd_client.mgmtd_server_addresses", `'["RDMA://10.16.28.58:8000"]'`,
		"--config.user_info.token", "AAA8WCoB8QAt8bFw2wBupzjA",
		`"upload-chain-table --desc stage 1 output/generated_chain_table.csv"`,
	}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))
}

func TestCreateChainAndTargetModelStepSuite(t *testing.T) {
	suiteRun(t, &createChainAndTargetModelStepSuite{})
}

type createChainAndTargetModelStepSuite struct {
	ttask.StepSuite

	node1 *model.Node
	node2 *model.Node
	stor1 *model.StorService
	stor2 *model.StorService
	disk1 *model.Disk
	disk2 *model.Disk
	disk3 *model.Disk
	step  *createChainAndTargetModelStep
}

func (s *createChainAndTargetModelStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.SetupRuntime()
	s.node1 = &model.Node{
		Name: "node1",
	}
	db := s.NewDB()
	s.NoError(db.Model(new(model.Node)).Create(s.node1).Error)
	s.node2 = &model.Node{
		Name: "node2",
	}
	s.NoError(db.Model(new(model.Node)).Create(s.node2).Error)
	s.stor1 = &model.StorService{
		NodeID:   s.node1.ID,
		FsNodeID: 10001,
	}
	s.NoError(db.Model(new(model.StorService)).Create(s.stor1).Error)
	s.stor2 = &model.StorService{
		NodeID:   s.node2.ID,
		FsNodeID: 10002,
	}
	s.NoError(db.Model(new(model.StorService)).Create(s.stor2).Error)
	s.disk1 = &model.Disk{
		Name:          "disk1",
		NodeID:        s.node1.ID,
		StorServiceID: s.stor1.ID,
		Index:         0,
	}
	s.NoError(db.Model(new(model.Disk)).Create(s.disk1).Error)
	s.disk2 = &model.Disk{
		Name:          "disk2",
		NodeID:        s.node2.ID,
		StorServiceID: s.stor2.ID,
		Index:         0,
	}
	s.NoError(db.Model(new(model.Disk)).Create(s.disk2).Error)
	s.disk3 = &model.Disk{
		Name:          "disk3",
		NodeID:        s.node2.ID,
		StorServiceID: s.stor2.ID,
		Index:         1,
	}
	s.NoError(db.Model(new(model.Disk)).Create(s.disk3).Error)

	s.step = &createChainAndTargetModelStep{}
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *createChainAndTargetModelStepSuite) TestCreate() {
	s.MockDocker.On("Exec", s.Cfg.Services.Mgmtd.ContainerName, "/opt/3fs/bin/admin_cli", []string{
		"--cfg", "/opt/3fs/etc/admin_cli.toml",
		`list-chains`,
	}).Return(`ChainId    ReferencedBy  ChainVersion  Status   PreferredOrder  Target                          Target
900100001  1       1             SERVING  []      101000100101(SERVING-UPTODATE)  101000200101(SERVING-UPTODATE)
900100002  1       1             SERVING  []      101000100102(SERVING-UPTODATE)  101000200102(SERVING-UPTODATE)`, nil)
	s.MockDocker.On("Exec", s.Cfg.Services.Mgmtd.ContainerName, "/opt/3fs/bin/admin_cli", []string{
		"--cfg", "/opt/3fs/etc/admin_cli.toml",
		`list-targets`,
	}).Return(`TargetId      ChainId    Role  PublicState  LocalState  NodeId  DiskIndex  UsedSize
101000100116  900100001  HEAD  SERVING      UPTODATE    10001   0          0
101000200116  900100002  TAIL  SERVING      UPTODATE    10002   0          0
101000100110  900100002  HEAD  SERVING      UPTODATE    10002   1          0`, nil)

	s.NoError(s.step.Execute(s.Ctx()))

	var chains []model.Chain
	db := s.NewDB()
	s.NoError(db.Model(new(model.Chain)).Order("id ASC").Find(&chains).Error)
	s.Len(chains, 2)
	chain1Exp := &model.Chain{
		Model: chains[0].Model,
		Name:  "900100001",
	}
	s.Equal(chain1Exp, &chains[0])
	chain2Exp := &model.Chain{
		Model: chains[1].Model,
		Name:  "900100002",
	}
	s.Equal(chain2Exp, &chains[1])

	var targets []model.Target
	s.NoError(db.Model(new(model.Target)).Order("id ASC").Find(&targets).Error)
	s.Len(targets, 3)
	target1Exp := &model.Target{
		Model:   targets[0].Model,
		Name:    "101000100116",
		DiskID:  s.disk1.ID,
		NodeID:  s.node1.ID,
		ChainID: chains[0].ID,
	}
	target2Exp := &model.Target{
		Model:   targets[1].Model,
		Name:    "101000200116",
		DiskID:  s.disk2.ID,
		NodeID:  s.node2.ID,
		ChainID: chains[1].ID,
	}
	target3Exp := &model.Target{
		Model:   targets[2].Model,
		Name:    "101000100110",
		DiskID:  s.disk3.ID,
		NodeID:  s.node2.ID,
		ChainID: chains[1].ID,
	}
	s.Equal(target1Exp, &targets[0])
	s.Equal(target2Exp, &targets[1])
	s.Equal(target3Exp, &targets[2])

	s.MockDocker.AssertExpectations(s.T())
}

func (s *createChainAndTargetModelStepSuite) TestWithTargetStorServiceNotFound() {
	s.MockDocker.On("Exec", s.Cfg.Services.Mgmtd.ContainerName, "/opt/3fs/bin/admin_cli", []string{
		"--cfg", "/opt/3fs/etc/admin_cli.toml",
		`list-chains`,
	}).Return(`ChainId    ReferencedBy  ChainVersion  Status   PreferredOrder  Target                          Target
900100002  1      1        SERVING  []       101000100102(SERVING-UPTODATE)  101000200102(SERVING-UPTODATE)`, nil)
	s.MockDocker.On("Exec", s.Cfg.Services.Mgmtd.ContainerName, "/opt/3fs/bin/admin_cli", []string{
		"--cfg", "/opt/3fs/etc/admin_cli.toml",
		`list-targets`,
	}).Return(`TargetId      ChainId    Role  PublicState  LocalState  NodeId  DiskIndex  UsedSize
101000100110  900100002  HEAD  SERVING      UPTODATE    10003   1          0`, nil)

	s.Error(s.step.Execute(s.Ctx()), "storage service not found")

	s.MockDocker.AssertExpectations(s.T())
}

func (s *createChainAndTargetModelStepSuite) TestWithTargetDiskNotFound() {
	s.MockDocker.On("Exec", s.Cfg.Services.Mgmtd.ContainerName, "/opt/3fs/bin/admin_cli", []string{
		"--cfg", "/opt/3fs/etc/admin_cli.toml",
		`list-chains`,
	}).Return(`ChainId    ReferencedBy  ChainVersion  Status   PreferredOrder  Target                          Target
900100002  1     1      SERVING  []       101000100102(SERVING-UPTODATE)  101000200102(SERVING-UPTODATE)`, nil)
	s.MockDocker.On("Exec", s.Cfg.Services.Mgmtd.ContainerName, "/opt/3fs/bin/admin_cli", []string{
		"--cfg", "/opt/3fs/etc/admin_cli.toml",
		`list-targets`,
	}).Return(`TargetId      ChainId    Role  PublicState  LocalState  NodeId  DiskIndex  UsedSize
101000100110  900100002  HEAD  SERVING      UPTODATE    10002   3          0`, nil)

	s.Error(s.step.Execute(s.Ctx()), "disk not found for storage service")

	s.MockDocker.AssertExpectations(s.T())
}
