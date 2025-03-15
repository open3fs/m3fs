package steps

import (
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/image"
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
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0])
}

func (s *gen3FSNodeIDStepSuite) TestGenNodeID() {
	s.NoError(s.step.Execute(s.Ctx()))

	idI, ok := s.Runtime.Load(getNodeIDKey("mgmtd_main", s.Cfg.Nodes[0].Name))
	s.True(ok)
	s.Equal(1, idI.(int))

	idI2, ok := s.Runtime.Load(getNodeIDKey("mgmtd_main", s.Cfg.Nodes[1].Name))
	s.True(ok)
	s.Equal(2, idI2.(int))
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
	s.node = s.Cfg.Nodes[0]
	s.Cfg.Services.Mgmtd.Nodes = []string{"node1"}
	s.Cfg.Services.Mgmtd.WorkDir = "/root"
	s.Cfg.Services.Mgmtd.TCPListenPort = 9000
	s.Cfg.Services.Mgmtd.RDMAListenPort = 8000
	s.SetupRuntime()

	s.step = NewPrepare3FSConfigStepFunc(&Prepare3FSConfigStepSetup{
		Service:        "mgmtd_main",
		ServiceWorkDir: "/root",
		TCPListenPort:  9000,
		RDMAListenPort: 8000,
		MainAppTomlTmpl: []byte(`allow_empty_node_id = true
node_id = {{ .NodeID }}`),
		MainLauncherTomlTmpl: []byte(`allow_dev_version = true
cluster_id = '{{ .ClusterID }}'
mgmtd_server_addresses = {{ .MgmtdServerAddresses }}`),
		MainTomlTmpl: []byte(`monitor_remote_ip = "{{ .MonitorRemoteIP }}"
mgmtd_server_addresses = {{ .MgmtdServerAddresses }}
listen_port = {{ .TCPListenPort }}
listen_port_rdma = {{ .RDMAListenPort }}`),
	})().(*prepare3FSConfigStep)
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0])
	s.Runtime.Store(getNodeIDKey("mgmtd_main", s.Cfg.Nodes[0].Name), 1)
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
	mainContent := `monitor_remote_ip = ""
mgmtd_server_addresses = 
listen_port = 9000
listen_port_rdma = 8000`
	adminCli := `admin_cli`
	return mainApp, mainLauncher, mainContent, adminCli
}

func (s *prepare3FSConfigStepSuite) testPrepareConfig(removeAllErr error) {
	s.MockLocalFS.On("MkdirAll", s.Runtime.Services.Mgmtd.WorkDir).Return(nil)
	tmpDir := "/root/tmp..."
	s.MockLocalFS.On("MkdirTemp", s.Runtime.Services.Mgmtd.WorkDir, s.node.Name+".*").
		Return(tmpDir, nil)
	s.MockLocalFS.On("RemoveAll", tmpDir).Return(removeAllErr)
	mainAppConfig, mainLauncherConfig, mainConfig, adminCli := s.getGeneratedConfigContent()
	s.mockGenConfig(tmpDir+"/mgmtd_main_app.toml", mainAppConfig)
	s.mockGenConfig(tmpDir+"/mgmtd_main_launcher.toml", mainLauncherConfig)
	s.mockGenConfig(tmpDir+"/mgmtd_main.toml", mainConfig)
	s.mockGenConfig(tmpDir+"/admin_cli.toml", adminCli)
	s.MockLocalFS.On("WriteFile", tmpDir+"/fdb.cluster", []byte(s.fdbContent), os.FileMode(0644)).
		Return(nil)
	s.MockRunner.On("Exec", "mkdir", []string{"-p", s.Runtime.Services.Mgmtd.WorkDir}).
		Return("", nil)
	s.MockRunner.On("Scp", tmpDir, s.Cfg.Services.Mgmtd.WorkDir+"/config.d").Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockLocalFS.AssertExpectations(s.T())
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
}

func (s *run3FSContainerStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.Cfg.Services.Mgmtd.WorkDir = "/var/mgmtd"
	s.configDir = "/var/mgmtd/config.d"
	s.SetupRuntime()
	s.step = NewRun3FSContainerStepFunc(
		&Run3FSContainerStepSetup{
			ImgName:       "3fs",
			ContainerName: s.Runtime.Services.Mgmtd.ContainerName,
			Service:       "mgmtd_main",
			WorkDir:       s.Runtime.Services.Mgmtd.WorkDir,
		})().(*run3FSContainerStep)
	s.step.Init(s.Runtime, s.MockEm, config.Node{})
	s.Runtime.Store(task.RuntimeFdbClusterFileContentKey, "xxxx")
}

func (s *run3FSContainerStepSuite) TestRunContainer() {
	img, err := image.GetImage(s.Runtime.Cfg.Registry.CustomRegistry, "3fs")
	s.NoError(err)
	s.MockDocker.On("Run", &external.RunArgs{
		Image:       img,
		Name:        &s.Cfg.Services.Mgmtd.ContainerName,
		Detach:      common.Pointer(true),
		HostNetwork: true,
		Privileged:  common.Pointer(true),
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
				Source: s.configDir,
				Target: "/opt/3fs/etc/",
			},
		},
	}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockRunner.AssertExpectations(s.T())
	s.MockDocker.AssertExpectations(s.T())
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

	s.Cfg.Services.Mgmtd.WorkDir = "/root/mgmtd"
	s.configDir = "/root/mgmtd/config.d"
	s.SetupRuntime()
	s.step = NewRm3FSContainerStepFunc(s.Cfg.Services.Mgmtd.ContainerName,
		"mgmtd_main", s.Cfg.Services.Mgmtd.WorkDir)().(*rm3FSContainerStep)
	s.step.Init(s.Runtime, s.MockEm, config.Node{})
}

func (s *rm3FSContainerStepSuite) TestRmContainerStep() {
	s.MockDocker.On("Rm", s.Cfg.Services.Mgmtd.ContainerName, true).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-rf", s.configDir}).Return("", nil)

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

	s.Cfg.Services.Meta.WorkDir = "/var/meta"
	s.configDir = "/var/meta/config.d"
	s.SetupRuntime()
	s.step = NewUpload3FSMainConfigStepFunc("3fs", s.Cfg.Services.Meta.ContainerName,
		"meta_main", s.Cfg.Services.Meta.WorkDir, "META")().(*upload3FSMainConfigStep)
	s.step.Init(s.Runtime, s.MockEm, config.Node{})
	s.Runtime.Store(task.RuntimeMgmtdServerAddressesKey, `["RDMA://1.1.1.1:8000"]`)
}

func (s *upload3FSMainConfigStepSuite) TestUploadConfig() {
	img, err := image.GetImage(s.Runtime.Cfg.Registry.CustomRegistry, "3fs")
	s.NoError(err)
	s.MockDocker.On("Run", &external.RunArgs{
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
		},
	}).Return("", nil)

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
	s.Cfg.Services.Storage.WorkDir = "/root"
	s.Cfg.Services.Storage.TCPListenPort = 9000
	s.Cfg.Services.Storage.RDMAListenPort = 8000
	s.SetupRuntime()

	s.step = NewRemoteRunScriptStepFunc(
		s.Cfg.Services.Storage.WorkDir,
		"test123",
		[]byte("ls -al"),
		[]string{
			"a", "b",
		},
	)().(*remoteRunScriptStep)
	s.step.Init(s.Runtime, s.MockEm, s.Cfg.Nodes[0])
	s.Runtime.Store(getNodeIDKey("storage_main", s.Cfg.Nodes[0].Name), 1)
	s.fdbContent = "xxxx,xxxxx,xxxx"
	s.Runtime.Store(task.RuntimeFdbClusterFileContentKey, s.fdbContent)
	s.Runtime.Store(task.RuntimeAdminCliTomlKey, []byte("admin_cli"))
}

func (s *remoteRunScriptStepSuite) testPrepareConfig(removeAllErr error) {
	s.MockLocalFS.On("MkdirAll", s.Runtime.Services.Storage.WorkDir).Return(nil)
	tmpDir := "/root/tmp..."
	s.MockLocalFS.On("MkdirTemp", s.Runtime.Services.Storage.WorkDir, s.node.Name+".*").
		Return(tmpDir, nil)
	s.MockLocalFS.On("RemoveAll", tmpDir).Return(removeAllErr)
	tmpFilePath := tmpDir + "/tmp_script.sh"
	s.MockLocalFS.On("WriteFile", tmpFilePath, []byte("ls -al"), os.FileMode(0775)).
		Return(nil)
	s.MockRunner.On("Exec", "mkdir", []string{"-p", s.Cfg.Services.Storage.WorkDir}).
		Return("", nil)
	s.MockRunner.On("Exec", "mktemp", []string{"-p", s.Cfg.Services.Storage.WorkDir}).
		Return(tmpFilePath, nil)
	s.MockRunner.On("Scp", tmpFilePath, tmpFilePath).Return(nil)
	s.MockRunner.On("Exec", "bash", []string{tmpFilePath, "a", "b"}).Return("", nil)
	s.MockRunner.On("Exec", "rm", []string{"-f", tmpFilePath}).Return("", nil)

	s.NoError(s.step.Execute(s.Ctx()))

	s.MockLocalFS.AssertExpectations(s.T())
	s.MockRunner.AssertExpectations(s.T())
}

func (s *remoteRunScriptStepSuite) TestRun() {
	s.testPrepareConfig(nil)
}

func (s *remoteRunScriptStepSuite) TestRunWithRmFailed() {
	s.testPrepareConfig(errors.New("dummy error"))
}
