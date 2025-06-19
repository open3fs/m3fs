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

	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
	ttask "github.com/open3fs/m3fs/tests/task"
)

var suiteRun = suite.Run

func TestGenAdminCliConfigSuite(t *testing.T) {
	suiteRun(t, &genAdminCliConfigStepSuite{})
}

type genAdminCliConfigStepSuite struct {
	ttask.StepSuite

	step *genAdminCliConfigStep
}

func (s *genAdminCliConfigStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.Cfg.Name = "test-cluster"
	s.SetupRuntime()
	s.step = &genAdminCliConfigStep{}
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *genAdminCliConfigStepSuite) Test() {
	s.NoError(s.step.Execute(s.Ctx()))

	dataI, ok := s.Runtime.Load(task.RuntimeAdminCliTomlKey)
	s.True(ok)
	s.Equal([]byte(`break_multi_line_command_on_failure = false
cluster_id = 'test-cluster'
log = 'DBG:normal; normal=file:path=/var/log/3fs/cli.log,async=true,sync_level=ERR'
num_timeout_ms = 1000
profile = false
verbose = false

[client]
default_compression_level = 0
default_compression_threshold = '128KB'
default_log_long_running_threshold = '0ns'
default_report_metrics = false
default_send_retry_times = 1
default_timeout = '1s'
enable_rdma_control = false
force_use_tcp = false

[client.io_worker]
num_event_loop = 1
rdma_connect_timeout = '5s'
read_write_rdma_in_event_thread = false
read_write_tcp_in_event_thread = false
tcp_connect_timeout = '1s'
wait_to_retry_send = '100ms'

[client.io_worker.connect_concurrency_limiter]
max_concurrency = 4

[client.io_worker.ibsocket]
buf_ack_batch = 8
buf_signal_batch = 8
buf_size = 16384
drain_timeout = '5s'
drop_connections = 0
event_ack_batch = 128
max_rd_atomic = 16
max_rdma_wr = 128
max_rdma_wr_per_post = 32
max_sge = 1
min_rnr_timer = 1
record_bytes_per_peer = false
record_latency_per_peer = false
retry_cnt = 7
rnr_retry = 0
send_buf_cnt = 32
sl = 0
start_psn = 0
timeout = 14

[client.io_worker.transport_pool]
max_connections = 1

[client.processor]
enable_coroutines_pool = true
max_coroutines_num = 256
max_processing_requests_num = 4096
response_compression_level = 1
response_compression_threshold = '128KB'

[client.rdma_control]
max_concurrent_transmission = 64

[client.thread_pool]
bg_thread_pool_stratetry = 'SHARED_QUEUE'
collect_stats = false
enable_work_stealing = false
io_thread_pool_stratetry = 'SHARED_QUEUE'
num_bg_threads = 2
num_connect_threads = 2
num_io_threads = 2
num_proc_threads = 2
proc_thread_pool_stratetry = 'SHARED_QUEUE'

[fdb]
casual_read_risky = false
clusterFile = '/opt/3fs/etc/fdb.cluster'
default_backoff = 0
enableMultipleClient = false
externalClientDir = ''
externalClientPath = ''
multipleClientThreadNum = 4
readonly = false
trace_file = ''
trace_format = 'json'

[ib_devices]
allow_no_usable_devices = false
allow_unknown_zone = true
default_network_zone = 'UNKNOWN'
default_pkey_index = 0
default_roce_pkey_index = 0
default_traffic_class = 0
device_filter = []
fork_safe = true
prefer_ibdevice = true
skip_inactive_ports = true
skip_unusable_device = true
subnets = []

[meta_client]
check_server_interval = '5s'
dynamic_stripe = false
max_concurrent_requests = 128
network_type = 'RDMA'
remove_chunks_batch_size = 32
remove_chunks_max_iters = 1024
selection_mode = 'RandomFollow'

[meta_client.background_closer]
prune_session_batch_count = 128
prune_session_batch_interval = '10s'
retry_first_wait = '100ms'
retry_max_wait = '10s'
task_scan = '50ms'

[meta_client.background_closer.coroutine_pool]
coroutines_num = 8
enable_work_stealing = false
queue_size = 128

[meta_client.retry_default]
max_failures_before_failover = 1
retry_fast = '1s'
retry_init_wait = '500ms'
retry_max_wait = '5s'
retry_send = 1
retry_total_time = '1min'
rpc_timeout = '5s'

[mgmtd_client]
accept_incomplete_routing_info_during_mgmtd_bootstrapping = true
auto_extend_client_session_interval = '10s'
auto_heartbeat_interval = '10s'
auto_refresh_interval = '1s'
enable_auto_extend_client_session = false
enable_auto_heartbeat = false
enable_auto_refresh = true
mgmtd_server_addresses = []
work_queue_size = 100

[monitor]
collect_period = '1s'
num_collectors = 1
reporters = []

[storage_client]
check_overlapping_read_buffers = true
check_overlapping_write_buffers = false
chunk_checksum_type = 'CRC32C'
create_net_client_for_updates = false
implementation_type = 'RPC'
max_inline_read_bytes = '0'
max_inline_write_bytes = '0'
max_read_io_bytes = '0'

[storage_client.net_client]
default_compression_level = 0
default_compression_threshold = '128KB'
default_log_long_running_threshold = '0ns'
default_report_metrics = false
default_send_retry_times = 1
default_timeout = '1s'
enable_rdma_control = false
force_use_tcp = false

[storage_client.net_client.io_worker]
num_event_loop = 1
rdma_connect_timeout = '5s'
read_write_rdma_in_event_thread = false
read_write_tcp_in_event_thread = false
tcp_connect_timeout = '1s'
wait_to_retry_send = '100ms'

[storage_client.net_client.io_worker.connect_concurrency_limiter]
max_concurrency = 4

[storage_client.net_client.io_worker.ibsocket]
buf_ack_batch = 8
buf_signal_batch = 8
buf_size = 16384
drain_timeout = '5s'
drop_connections = 0
event_ack_batch = 128
max_rd_atomic = 16
max_rdma_wr = 128
max_rdma_wr_per_post = 32
max_sge = 1
min_rnr_timer = 1
record_bytes_per_peer = false
record_latency_per_peer = false
retry_cnt = 7
rnr_retry = 0
send_buf_cnt = 32
sl = 0
start_psn = 0
timeout = 14

[storage_client.net_client.io_worker.transport_pool]
max_connections = 1

[storage_client.net_client.processor]
enable_coroutines_pool = true
max_coroutines_num = 256
max_processing_requests_num = 4096
response_compression_level = 1
response_compression_threshold = '128KB'

[storage_client.net_client.rdma_control]
max_concurrent_transmission = 64

[storage_client.net_client.thread_pool]
bg_thread_pool_stratetry = 'SHARED_QUEUE'
collect_stats = false
enable_work_stealing = false
io_thread_pool_stratetry = 'SHARED_QUEUE'
num_bg_threads = 2
num_connect_threads = 2
num_io_threads = 2
num_proc_threads = 2
proc_thread_pool_stratetry = 'SHARED_QUEUE'

[storage_client.net_client_for_updates]
default_compression_level = 0
default_compression_threshold = '128KB'
default_log_long_running_threshold = '0ns'
default_report_metrics = false
default_send_retry_times = 1
default_timeout = '1s'
enable_rdma_control = false
force_use_tcp = false

[storage_client.net_client_for_updates.io_worker]
num_event_loop = 1
rdma_connect_timeout = '5s'
read_write_rdma_in_event_thread = false
read_write_tcp_in_event_thread = false
tcp_connect_timeout = '1s'
wait_to_retry_send = '100ms'

[storage_client.net_client_for_updates.io_worker.connect_concurrency_limiter]
max_concurrency = 4

[storage_client.net_client_for_updates.io_worker.ibsocket]
buf_ack_batch = 8
buf_signal_batch = 8
buf_size = 16384
drain_timeout = '5s'
drop_connections = 0
event_ack_batch = 128
max_rd_atomic = 16
max_rdma_wr = 128
max_rdma_wr_per_post = 32
max_sge = 1
min_rnr_timer = 1
record_bytes_per_peer = false
record_latency_per_peer = false
retry_cnt = 7
rnr_retry = 0
send_buf_cnt = 32
sl = 0
start_psn = 0
timeout = 14

[storage_client.net_client_for_updates.io_worker.transport_pool]
max_connections = 1

[storage_client.net_client_for_updates.processor]
enable_coroutines_pool = true
max_coroutines_num = 256
max_processing_requests_num = 4096
response_compression_level = 1
response_compression_threshold = '128KB'

[storage_client.net_client_for_updates.rdma_control]
max_concurrent_transmission = 64

[storage_client.net_client_for_updates.thread_pool]
bg_thread_pool_stratetry = 'SHARED_QUEUE'
collect_stats = false
enable_work_stealing = false
io_thread_pool_stratetry = 'SHARED_QUEUE'
num_bg_threads = 2
num_connect_threads = 2
num_io_threads = 2
num_proc_threads = 2
proc_thread_pool_stratetry = 'SHARED_QUEUE'

[storage_client.retry]
init_wait_time = '10s'
max_failures_before_failover = 1
max_retry_time = '1min'
max_wait_time = '30s'

[storage_client.traffic_control.query]
max_batch_bytes = '4MB'
max_batch_size = 128
max_concurrent_requests = 32
max_concurrent_requests_per_server = 8
process_batches_in_parallel = true
random_shuffle_requests = true

[storage_client.traffic_control.read]
max_batch_bytes = '4MB'
max_batch_size = 128
max_concurrent_requests = 32
max_concurrent_requests_per_server = 8
process_batches_in_parallel = true
random_shuffle_requests = true

[storage_client.traffic_control.remove]
max_batch_bytes = '4MB'
max_batch_size = 128
max_concurrent_requests = 32
max_concurrent_requests_per_server = 8
process_batches_in_parallel = true
random_shuffle_requests = true

[storage_client.traffic_control.truncate]
max_batch_bytes = '4MB'
max_batch_size = 128
max_concurrent_requests = 32
max_concurrent_requests_per_server = 8
process_batches_in_parallel = true
random_shuffle_requests = true

[storage_client.traffic_control.write]
max_batch_bytes = '4MB'
max_batch_size = 128
max_concurrent_requests = 32
max_concurrent_requests_per_server = 8
process_batches_in_parallel = true
random_shuffle_requests = true

[user_info]
gid = -1
gids = []
token = ''
uid = -1`), dataI.([]byte))
}

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

func (s *createChainAndTargetModelStepSuite) testCreate(isDir bool) {
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
	if isDir {
		target1Exp.DiskID = 0
		target2Exp.DiskID = 0
		target3Exp.DiskID = 0
	}
	s.Equal(target1Exp, &targets[0])
	s.Equal(target2Exp, &targets[1])
	s.Equal(target3Exp, &targets[2])

	s.MockDocker.AssertExpectations(s.T())
}

func (s *createChainAndTargetModelStepSuite) TestCreateWithNvme() {
	s.testCreate(false)
}

func (s *createChainAndTargetModelStepSuite) TestCreateWithDir() {
	s.Runtime.Cfg.Services.Storage.DiskType = config.DiskTypeDirectory
	s.testCreate(true)
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
