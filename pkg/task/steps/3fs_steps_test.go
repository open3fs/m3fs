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

package steps

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/task"
	ttask "github.com/open3fs/m3fs/tests/task"
)

var suiteRun = suite.Run

func TestGen3FSNodeIDStepSuite(t *testing.T) {
	suiteRun(t, &gen3FSNodeIDStepSuite{})
}

type gen3FSNodeIDStepSuite struct {
	ttask.StepSuite

	step *gen3FSNodeIDStep
}

func (s *gen3FSNodeIDStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
		{
			Name: "node2",
			Host: "1.1.1.2",
		},
	}
	s.Cfg.Services.Mgmtd.Nodes = []string{"node1", "node2"}
	s.SetupRuntime()
	s.step = NewGen3FSNodeIDStepFunc("mgmtd_main",
		1, s.Cfg.Services.Mgmtd.Nodes)().(*gen3FSNodeIDStep)
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
}

func (s *gen3FSNodeIDStepSuite) TestGenNodeID() {
	s.NoError(s.step.Execute(s.Ctx()))

	idI, ok := s.Runtime.Load(GetNodeIDKey("mgmtd_main", s.Cfg.Nodes[0].Name))
	s.True(ok)
	s.Equal(1, idI.(int))

	idI2, ok := s.Runtime.Load(GetNodeIDKey("mgmtd_main", s.Cfg.Nodes[1].Name))
	s.True(ok)
	s.Equal(2, idI2.(int))
}

func TestGenAdminCliConfigSuite(t *testing.T) {
	suiteRun(t, &genAdminCliConfigStepSuite{})
}

type genAdminCliConfigStepSuite struct {
	ttask.StepSuite

	step *genAdminCliConfigStep
}

func (s *genAdminCliConfigStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.Cfg.Name = "test"
	s.SetupRuntime()
	s.step = &genAdminCliConfigStep{}
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *genAdminCliConfigStepSuite) Test() {
	s.NoError(s.step.Execute(s.Ctx()))

	dataI, ok := s.Runtime.Load(task.RuntimeAdminCliTomlKey)
	s.True(ok)
	s.Equal([]byte(`break_multi_line_command_on_failure = false
cluster_id = 'test'
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

func TestPrepare3FSConfigStepSuite(t *testing.T) {
	suiteRun(t, &prepare3FSConfigStepSuite{})
}

type prepare3FSConfigStepSuite struct {
	ttask.StepSuite

	step       *prepare3FSConfigStep
	node       config.Node
	fdbContent string
}

func (s *prepare3FSConfigStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
	}
	s.Cfg.Name = "test-cluster"
	s.Cfg.LogLevel = "DEBUG"
	s.node = s.Cfg.Nodes[0]
	s.Cfg.Services.Mgmtd.Nodes = []string{"node1"}
	s.Cfg.Services.Mgmtd.TCPListenPort = 9000
	s.Cfg.Services.Mgmtd.RDMAListenPort = 8000
	s.SetupRuntime()

	s.step = NewPrepare3FSConfigStepFunc(&Prepare3FSConfigStepSetup{
		Service:        "mgmtd_main",
		ServiceWorkDir: "/root/3fs/mgmtd",
		TCPListenPort:  9000,
		RDMAListenPort: 8000,
		MainAppTomlTmpl: []byte(`allow_empty_node_id = true
node_id = {{ .NodeID }}`),
		MainLauncherTomlTmpl: []byte(`allow_dev_version = true
cluster_id = '{{ .ClusterID }}'
mgmtd_server_addresses = {{ .MgmtdServerAddresses }}`),
		MainTomlTmpl: []byte(`level = "{{ .LogLevel }}"
monitor_remote_ip = "{{ .MonitorRemoteIP }}"
mgmtd_server_addresses = {{ .MgmtdServerAddresses }}
listen_port = {{ .TCPListenPort }}
listen_port_rdma = {{ .RDMAListenPort }}`),
	})().(*prepare3FSConfigStep)
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
	s.Runtime.Store(GetNodeIDKey("mgmtd_main", s.Cfg.Nodes[0].Name), 1)
	s.fdbContent = "xxxx,xxxxx,xxxx"
	s.Runtime.Store(task.RuntimeFdbClusterFileContentKey, s.fdbContent)
	s.Runtime.Store(task.RuntimeAdminCliTomlKey, []byte("admin_cli"))
}

func (s *prepare3FSConfigStepSuite) mockGenConfig(path, tmpContent string) {
	var data any = mock.AnythingOfType("[]uint8")
	if tmpContent != "" {
		data = []byte(tmpContent)
	}
	s.MockLocalFS.On("WriteFile", path, data, os.FileMode(0644)).Return(nil)
}

func (s *prepare3FSConfigStepSuite) getGeneratedConfigContent() (string, string, string, string) {
	mainApp := `allow_empty_node_id = true
node_id = 1`
	mainLauncher := `allow_dev_version = true
cluster_id = 'test-cluster'
mgmtd_server_addresses = `
	mainContent := `level = "DEBUG"
monitor_remote_ip = ""
mgmtd_server_addresses = 
listen_port = 9000
listen_port_rdma = 8000`
	adminCli := `admin_cli`
	return mainApp, mainLauncher, mainContent, adminCli
}

func (s *prepare3FSConfigStepSuite) testPrepareConfig(removeAllErr error) {
	tmpDir := "/root/tmp..."
	s.MockLocalFS.On("MkdirTemp", "/tmp", "prepare-3fs-config").Return(tmpDir, nil)
	s.MockLocalFS.On("RemoveAll", tmpDir).Return(removeAllErr)
	mainAppConfig, mainLauncherConfig, mainConfig, adminCli := s.getGeneratedConfigContent()
	s.mockGenConfig(tmpDir+"/mgmtd_main_app.toml", mainAppConfig)
	s.mockGenConfig(tmpDir+"/mgmtd_main_launcher.toml", mainLauncherConfig)
	s.mockGenConfig(tmpDir+"/mgmtd_main.toml", mainConfig)
	s.mockGenConfig(tmpDir+"/admin_cli.toml", adminCli)
	s.MockLocalFS.On("WriteFile", tmpDir+"/fdb.cluster", []byte(s.fdbContent), os.FileMode(0644)).
		Return(nil)
	s.MockFS.On("MkdirAll", "/root/3fs/mgmtd").Return(nil)
	s.MockRunner.On("Scp", tmpDir, "/root/3fs/mgmtd/config.d").Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockLocalFS.AssertExpectations(s.T())
	s.MockFS.AssertExpectations(s.T())
	s.MockRunner.AssertExpectations(s.T())
}

func (s *prepare3FSConfigStepSuite) TestPrepareConfig() {
	s.testPrepareConfig(nil)
}

func (s *prepare3FSConfigStepSuite) TestPrepareConfigWithRemoveTempDirFailed() {
	s.testPrepareConfig(errors.New("remove temp dir failed"))
}

func TestRun3FSContainerStepSuite(t *testing.T) {
	suiteRun(t, &run3FSContainerStepSuite{})
}

type run3FSContainerStepSuite struct {
	ttask.StepSuite

	step      *run3FSContainerStep
	configDir string
	logDir    string
}

func (s *run3FSContainerStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.configDir = "/root/3fs/mgmtd/config.d"
	s.logDir = "/root/3fs/mgmtd/log"
	s.Cfg.Nodes = append(s.Cfg.Nodes, config.Node{
		Name: "test-node",
		Host: "1.1.1.1",
	})
	s.Cfg.Services.Mgmtd.Nodes = []string{"test-node"}
	s.SetupRuntime()
	s.step = NewRun3FSContainerStepFunc(
		&Run3FSContainerStepSetup{
			ImgName:       config.ImageName3FS,
			ContainerName: s.Runtime.Services.Mgmtd.ContainerName,
			Service:       "mgmtd_main",
			WorkDir:       "/root/3fs/mgmtd",
			ModelObjFunc: func(s *task.BaseStep) any {
				fsNodeID, _ := s.Runtime.LoadInt(
					GetNodeIDKey("mgmtd_main", s.Node.Name))
				return &model.MgmtService{
					Name:     s.Runtime.Services.Mgmtd.ContainerName,
					NodeID:   s.GetNodeModelID(),
					FsNodeID: fmt.Sprintf("%d", fsNodeID),
				}
			},
		})().(*run3FSContainerStep)
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
	s.Runtime.Store(task.RuntimeFdbClusterFileContentKey, "xxxx")
}

func (s *run3FSContainerStepSuite) testRunContainer(
	useRdmaNetwork bool, networkType config.NetworkType) {
	s.step.useRdmaNetwork = useRdmaNetwork
	s.step.Runtime.Cfg.NetworkType = networkType
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageName3FS)
	s.NoError(err)
	args := &external.RunArgs{
		Image:         img,
		Name:          &s.Cfg.Services.Mgmtd.ContainerName,
		Detach:        common.Pointer(true),
		RestartPolicy: external.ContainerRestartPolicyUnlessStepped,
		HostNetwork:   true,
		Privileged:    common.Pointer(true),
		Ulimits: map[string]string{
			"nofile": "1048576:1048576",
		},
		Command: []string{
			"/opt/3fs/bin/mgmtd_main",
			"--launcher_cfg", "/opt/3fs/etc/mgmtd_main_launcher.toml",
			"--app_cfg", "/opt/3fs/etc/mgmtd_main_app.toml",
		},
		Volumes: []*external.VolumeArgs{
			{
				Source: "/dev",
				Target: "/dev",
			},
			{
				Source: s.configDir,
				Target: "/opt/3fs/etc/",
			},
			{
				Source: s.logDir,
				Target: "/var/log/3fs",
			},
		},
	}
	if useRdmaNetwork {
		s.Runtime.Store(s.step.GetErdmaSoPathKey(),
			"/usr/lib/x86_64-linux-gnu/libibverbs/liberdma-rdmav34.so")
		args.Volumes = append(args.Volumes, s.step.GetRdmaVolumes()...)
	}
	s.MockDocker.On("Run", args).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	var mgmtService model.MgmtService
	s.NoError(s.NewDB().Model(new(model.MgmtService)).First(&mgmtService).Error)
	mgmtServiceExp := model.MgmtService{
		Model:    mgmtService.Model,
		Name:     s.Runtime.Services.Mgmtd.ContainerName,
		NodeID:   s.Runtime.LoadNodesMap()[s.step.Node.Name].ID,
		FsNodeID: "0",
	}
	s.Equal(mgmtServiceExp, mgmtService)

	s.MockRunner.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func (s *run3FSContainerStepSuite) TestRunContainerWithRdmaNetwork() {
	s.testRunContainer(true, config.NetworkTypeRDMA)
}

func (s *run3FSContainerStepSuite) TestRunContainerWithErdmaNetwork() {
	s.testRunContainer(true, config.NetworkTypeERDMA)
}

func (s *run3FSContainerStepSuite) TestRunContainerWithRxeRdmaNetwork() {
	s.testRunContainer(true, config.NetworkTypeRXE)
}

func (s *run3FSContainerStepSuite) TestRunContainerWithoutRdmaNetwork() {
	s.testRunContainer(false, config.NetworkTypeRDMA)
}

func TestRm3FSContainerStepSuite(t *testing.T) {
	suiteRun(t, &rm3FSContainerStepSuite{})
}

type rm3FSContainerStepSuite struct {
	ttask.StepSuite

	step      *rm3FSContainerStep
	configDir string
}

func (s *rm3FSContainerStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.configDir = "/root/3fs/mgmtd/config.d"
	s.SetupRuntime()
	s.step = NewRm3FSContainerStepFunc(s.Cfg.Services.Mgmtd.ContainerName,
		"mgmtd_main", "/root/3fs/mgmtd")().(*rm3FSContainerStep)
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
}

func (s *rm3FSContainerStepSuite) TestRmContainerStep() {
	s.MockDocker.On("Rm", s.Cfg.Services.Mgmtd.ContainerName, true).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.configDir}).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", "/root/3fs/mgmtd/log"}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func (s *rm3FSContainerStepSuite) TestRmContainerFailed() {
	s.MockDocker.On("Rm", s.Cfg.Services.Mgmtd.ContainerName, true).
		Return("", errors.New("dummy error"))

	s.Error(s.step.Execute(s.Ctx()), "dummy error")

	s.MockDocker.AssertExpectations(s.T())
}

func (s *rm3FSContainerStepSuite) TestRmDirFailed() {
	s.MockDocker.On("Rm", s.Cfg.Services.Mgmtd.ContainerName, true).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.configDir}).
		Return("", errors.New("dummy error"))

	s.Error(s.step.Execute(s.Ctx()), "dummy error")

	s.MockRunner.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func TestUpload3FSMainConfigStepSuite(t *testing.T) {
	suiteRun(t, &upload3FSMainConfigStepSuite{})
}

type upload3FSMainConfigStepSuite struct {
	ttask.StepSuite

	step      *upload3FSMainConfigStep
	configDir string
}

func (s *upload3FSMainConfigStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.configDir = "/root/3fs/meta/config.d"
	s.SetupRuntime()
	s.step = NewUpload3FSMainConfigStepFunc(config.ImageName3FS, s.Cfg.Services.Meta.ContainerName,
		"meta_main", "/root/3fs/meta", "META")().(*upload3FSMainConfigStep)
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
	s.Runtime.Store(task.RuntimeMgmtdServerAddressesKey, `["RDMA://1.1.1.1:8000"]`)
}

func (s *upload3FSMainConfigStepSuite) TestUploadConfig() {
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageName3FS)
	s.NoError(err)
	args := &external.RunArgs{
		Image:       img,
		Name:        &s.Cfg.Services.Meta.ContainerName,
		HostNetwork: true,
		Privileged:  common.Pointer(true),
		Entrypoint:  common.Pointer("''"),
		Rm:          common.Pointer(true),
		Ulimits: map[string]string{
			"nofile": "1048576:1048576",
		},
		Command: []string{
			"/opt/3fs/bin/admin_cli",
			"-cfg", "/opt/3fs/etc/admin_cli.toml",
			"--config.mgmtd_client.mgmtd_server_addresses",
			`'["RDMA://1.1.1.1:8000"]'`,
			"'set-config --type META --file /opt/3fs/etc/meta_main.toml'",
		},
		Volumes: []*external.VolumeArgs{
			{
				Source: s.configDir,
				Target: "/opt/3fs/etc/",
			},
			{
				Source: "/dev",
				Target: "/dev",
			},
		},
	}
	s.Runtime.Store(s.step.GetErdmaSoPathKey(),
		"/usr/lib/x86_64-linux-gnu/libibverbs/liberdma-rdmav34.so")
	args.Volumes = append(args.Volumes, s.step.GetRdmaVolumes()...)

	s.MockDocker.On("Run", args).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
}

func TestRemoteRunScriptStepSuite(t *testing.T) {
	suiteRun(t, &remoteRunScriptStepSuite{})
}

type remoteRunScriptStepSuite struct {
	ttask.StepSuite

	step       *remoteRunScriptStep
	node       config.Node
	fdbContent string
}

func (s *remoteRunScriptStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.Cfg.Nodes = []config.Node{
		{
			Name: "node1",
			Host: "1.1.1.1",
		},
	}
	s.Cfg.Name = "test-cluster"
	s.node = s.Cfg.Nodes[0]
	s.Cfg.Services.Storage.Nodes = []string{"node1"}
	s.Cfg.Services.Storage.TCPListenPort = 9000
	s.Cfg.Services.Storage.RDMAListenPort = 8000
	s.SetupRuntime()

	s.step = NewRemoteRunScriptStepFunc(
		"/root/3fs/storage",
		"test123",
		[]byte("ls -al"),
		map[string]any{},
		[]string{
			"a", "b",
		},
	)().(*remoteRunScriptStep)
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0], s.Logger)
	s.Runtime.Store(GetNodeIDKey("storage_main", s.Cfg.Nodes[0].Name), 1)
	s.fdbContent = "xxxx,xxxxx,xxxx"
	s.Runtime.Store(task.RuntimeFdbClusterFileContentKey, s.fdbContent)
	s.Runtime.Store(task.RuntimeAdminCliTomlKey, []byte("admin_cli"))
}

func (s *remoteRunScriptStepSuite) testPrepareConfig(removeAllErr error) {
	tmpDir := "/root/tmp..."
	s.MockLocalFS.On("MkdirTemp", "/tmp", "remote-run-script").
		Return(tmpDir, nil)
	s.MockLocalFS.On("RemoveAll", tmpDir).Return(removeAllErr)
	tmpFilePath := tmpDir + "/tmp_script.sh"
	s.MockLocalFS.On("WriteFile", tmpFilePath, []byte("ls -al"), os.FileMode(0777)).
		Return(nil)
	s.MockFS.On("MkdirAll", "/root/3fs/storage").Return(nil)
	s.MockFS.On("MkTempFile", "/tmp").Return(tmpFilePath, nil)
	s.MockRunner.On("Scp", tmpFilePath, tmpFilePath).Return(nil)
	s.MockRunner.On("Exec", "bash", []string{tmpFilePath, "a", "b"}).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-f", tmpFilePath}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockLocalFS.AssertExpectations(s.T())
	s.MockFS.AssertExpectations(s.T())
	s.MockRunner.AssertExpectations(s.T())
}

func (s *remoteRunScriptStepSuite) TestRun() {
	s.testPrepareConfig(nil)
}

func (s *remoteRunScriptStepSuite) TestRunWithRmFailed() {
	s.testPrepareConfig(errors.New("dummy error"))
}
