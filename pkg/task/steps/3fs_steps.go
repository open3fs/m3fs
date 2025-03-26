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
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/task"
)

func getNodeIDKey(service, name string) string {
	return fmt.Sprintf("%s-node-%s-id-", service, name)
}

func getConfigDir(workDir string) string {
	return path.Join(workDir, "config.d")
}

type gen3FSNodeIDStep struct {
	task.BaseStep

	idBegin int
	service string
	nodes   []string
}

func (s *gen3FSNodeIDStep) Execute(context.Context) error {
	nodes := make([]config.Node, len(s.nodes))
	for i, nodeName := range s.nodes {
		nodes[i] = s.Runtime.Nodes[nodeName]
	}

	nodeIDMap := make(map[string]int, len(nodes))
	for i, node := range nodes {
		s.Runtime.Store(getNodeIDKey(s.service, node.Name), s.idBegin+i)
		nodeIDMap[node.Name] = s.idBegin + i
	}
	s.Logger.Debugf("Node ID map: %v", nodeIDMap)

	return nil
}

// NewGen3FSNodeIDStepFunc is the generate 3fs node id step factory func.
func NewGen3FSNodeIDStepFunc(service string, idBegin int, nodes []string) func() task.Step {
	return func() task.Step {
		return &gen3FSNodeIDStep{
			service: service,
			nodes:   nodes,
			idBegin: idBegin,
		}
	}
}

// Extra3FSConfigFile defines extra config file that custom by specific task
type Extra3FSConfigFile struct {
	Data     []byte
	FileName string
}

type prepare3FSConfigStep struct {
	task.BaseStep

	service              string
	serviceWorkDir       string
	mainAppTomlTmpl      []byte
	mainLauncherTomlTmpl []byte
	mainTomlTmpl         []byte
	rdmaListenPort       int
	tcpListenPort        int
	extraMainTomlData    map[string]any
	extraConfigFilesFunc func(*task.Runtime) []*Extra3FSConfigFile
}

func (s *prepare3FSConfigStep) getMoniterEndpoints() string {
	monitor := s.Runtime.Services.Monitor
	endpoints := make([]string, len(monitor.Nodes))
	for i, nodeName := range monitor.Nodes {
		node := s.Runtime.Nodes[nodeName]
		endpoints[i] = net.JoinHostPort(node.Host, strconv.Itoa(monitor.Port))
	}

	return strings.Join(endpoints, ",")
}

func (s *prepare3FSConfigStep) Execute(ctx context.Context) error {
	localEm := s.Runtime.LocalEm
	tmpDir, err := localEm.FS.MkdirTemp(ctx, os.TempDir(), "prepare-3fs-config")
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err := localEm.FS.RemoveAll(ctx, tmpDir); err != nil {
			s.Logger.Errorf("Failed to remove temporary directory %s: %v", tmpDir, err)
		}
	}()

	s.Logger.Infof("Create %s config dir %s", s.service, s.serviceWorkDir)
	if err = s.Em.FS.MkdirAll(ctx, s.serviceWorkDir); err != nil {
		return errors.Trace(err)
	}

	if err = s.genConfigs(tmpDir); err != nil {
		return errors.Trace(err)
	}

	if err = s.genFdbClusterFile(tmpDir); err != nil {
		return errors.Trace(err)
	}

	if err = s.copyFile(ctx, tmpDir); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (s *prepare3FSConfigStep) copyFile(ctx context.Context, src string) error {
	dst := getConfigDir(s.serviceWorkDir)
	s.Logger.Infof("Copying %s configs from %s to %s %s", s.service, src, s.Node.Name, dst)
	if err := s.Em.Runner.Scp(ctx, src, dst); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (s *prepare3FSConfigStep) genConfig(path, tmplName string, tmpl []byte, tmplData any) error {
	s.Logger.Infof("Generating %s to %s", tmplName, path)
	t, err := template.New(tmplName).Parse(string(tmpl))
	if err != nil {
		return errors.Annotatef(err, "parse template of %s", path)
	}
	data := new(bytes.Buffer)

	err = t.Execute(data, tmplData)
	if err != nil {
		return errors.Annotatef(err, "execute template of %s", path)
	}
	s.Logger.Debugf("Config of %s: %s", tmplName, data.String())

	err = s.Runtime.LocalEm.FS.WriteFile(path, data.Bytes(), 0644)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (s *prepare3FSConfigStep) genConfigs(tmpDir string) error {
	nodeID, _ := s.Runtime.LoadInt(getNodeIDKey(s.service, s.Node.Name))
	mgmtdServerAddresses, _ := s.Runtime.LoadString(task.RuntimeMgmtdServerAddressesKey)

	mainAppToml := path.Join(tmpDir, fmt.Sprintf("%s_app.toml", s.service))
	mainLauncherToml := path.Join(tmpDir, fmt.Sprintf("%s_launcher.toml", s.service))
	mainToml := path.Join(tmpDir, fmt.Sprintf("%s.toml", s.service))
	adminCliToml := path.Join(tmpDir, "admin_cli.toml")

	appTmplData := map[string]any{
		"NodeID": nodeID,
	}
	s.Logger.Debugf("Template data of %s_app.toml.tmpl: %v", s.service, appTmplData)
	if err := s.genConfig(mainAppToml, fmt.Sprintf("%s_app.toml", s.service),
		s.mainAppTomlTmpl, appTmplData); err != nil {

		return errors.Trace(err)
	}

	launcherTmplData := map[string]any{
		"ClusterID":            s.Runtime.Cfg.Name,
		"MgmtdServerAddresses": mgmtdServerAddresses,
	}
	s.Logger.Debugf("Template data of %s_launcher.toml.tmpl: %v", s.service, launcherTmplData)
	if err := s.genConfig(mainLauncherToml, fmt.Sprintf("%s_launcher.toml", s.service),
		s.mainLauncherTomlTmpl, launcherTmplData); err != nil {

		return errors.Trace(err)
	}

	mainTmplData := map[string]any{
		"LogLevel":             s.Runtime.Cfg.LogLevel,
		"MonitorRemoteIP":      s.getMoniterEndpoints(),
		"RDMAListenPort":       s.rdmaListenPort,
		"TCPListenPort":        s.tcpListenPort,
		"MgmtdServerAddresses": mgmtdServerAddresses,
	}
	for k, v := range s.extraMainTomlData {
		mainTmplData[k] = v
	}
	s.Logger.Debugf("Template data of %s.toml.tmpl: %v", s.service, mainTmplData)
	if err := s.genConfig(mainToml, fmt.Sprintf("%s.toml", s.service),
		s.mainTomlTmpl, mainTmplData); err != nil {

		return errors.Trace(err)
	}

	adminCliI, _ := s.Runtime.Load(task.RuntimeAdminCliTomlKey)
	adminCliTomlData := adminCliI.([]byte)
	s.Logger.Infof("Save admin cli config to %s", adminCliToml)
	err := s.Runtime.LocalEm.FS.WriteFile(adminCliToml, adminCliTomlData, os.FileMode(0644))
	if err != nil {
		return errors.Trace(err)
	}

	if s.extraConfigFilesFunc != nil {
		for _, extraCfg := range s.extraConfigFilesFunc(s.Runtime) {
			filePath := path.Join(tmpDir, extraCfg.FileName)
			s.Logger.Infof("Save %s to %s", extraCfg.FileName, filePath)
			if err = s.Runtime.LocalEm.FS.WriteFile(filePath, extraCfg.Data, os.FileMode(0644)); err != nil {
				return errors.Trace(err)
			}
		}
	}

	return nil
}

func (s *prepare3FSConfigStep) genFdbClusterFile(tmpDir string) error {
	content, _ := s.Runtime.LoadString(task.RuntimeFdbClusterFileContentKey)

	clusterFilePath := path.Join(tmpDir, "fdb.cluster")
	s.Logger.Infof("Generating fdb.cluster to %s", clusterFilePath)
	if err := s.Runtime.LocalEm.FS.WriteFile(clusterFilePath, []byte(content), 0644); err != nil {
		return errors.Trace(err)
	}

	return nil
}

// Prepare3FSConfigStepSetup is a struct that holds the configuration of the prepare3FSConfigStep.
type Prepare3FSConfigStepSetup struct {
	Service                 string
	ServiceWorkDir          string
	MainAppTomlTmpl         []byte
	MainLauncherTomlTmpl    []byte
	MainTomlTmpl            []byte
	RDMAListenPort          int
	TCPListenPort           int
	ExtraMainTomlData       map[string]any
	Extra3FSConfigFilesFunc func(*task.Runtime) []*Extra3FSConfigFile
}

// NewPrepare3FSConfigStepFunc is prepare 3fs config step factory func.
func NewPrepare3FSConfigStepFunc(setup *Prepare3FSConfigStepSetup) func() task.Step {
	return func() task.Step {
		return &prepare3FSConfigStep{
			service:              setup.Service,
			serviceWorkDir:       setup.ServiceWorkDir,
			mainAppTomlTmpl:      setup.MainAppTomlTmpl,
			mainLauncherTomlTmpl: setup.MainLauncherTomlTmpl,
			mainTomlTmpl:         setup.MainTomlTmpl,
			rdmaListenPort:       setup.RDMAListenPort,
			tcpListenPort:        setup.TCPListenPort,
			extraMainTomlData:    setup.ExtraMainTomlData,
			extraConfigFilesFunc: setup.Extra3FSConfigFilesFunc,
		}
	}
}

type run3FSContainerStep struct {
	task.BaseStep

	imgName        string
	containerName  string
	service        string
	serviceWorkDir string
	extraVolumes   []*external.VolumeArgs
	useRdmaNetwork bool
}

func (s *run3FSContainerStep) Execute(ctx context.Context) error {
	s.Logger.Infof("Starting %s container %s", s.service, s.containerName)
	img, err := s.Runtime.Cfg.Images.GetImage(s.imgName)
	if err != nil {
		return errors.Trace(err)
	}
	args := &external.RunArgs{
		Image:       img,
		Name:        &s.containerName,
		HostNetwork: true,
		Privileged:  common.Pointer(true),
		Ulimits: map[string]string{
			"nofile": "1048576:1048576",
		},
		Command: []string{
			fmt.Sprintf("/opt/3fs/bin/%s", s.service),
			"--launcher_cfg", fmt.Sprintf("/opt/3fs/etc/%s_launcher.toml", s.service),
			"--app_cfg", fmt.Sprintf("/opt/3fs/etc/%s_app.toml", s.service),
		},
		Detach: common.Pointer(true),
		Volumes: []*external.VolumeArgs{
			{
				Source: "/dev",
				Target: "/dev",
			},
			{
				Source: getConfigDir(s.serviceWorkDir),
				Target: "/opt/3fs/etc/",
			},
			{
				Source: path.Join(s.serviceWorkDir, "log"),
				Target: "/var/log/3fs",
			},
		},
	}
	args.Volumes = append(args.Volumes, s.extraVolumes...)

	if s.useRdmaNetwork {
		if err := s.GetErdmaSoPath(ctx); err != nil {
			return errors.Trace(err)
		}
		args.Volumes = append(args.Volumes, s.GetRdmaVolumes()...)
	}
	_, err = s.Em.Docker.Run(ctx, args)
	if err != nil {
		return errors.Trace(err)
	}

	s.Logger.Infof("Started %s container %s successfully", s.service, s.containerName)
	return nil
}

// Run3FSContainerStepSetup is a struct that holds the configuration of the run3FSContainerStep.
type Run3FSContainerStepSetup struct {
	ImgName        string
	ContainerName  string
	Service        string
	WorkDir        string
	ExtraVolumes   []*external.VolumeArgs
	UseRdmaNetwork bool
}

// NewRun3FSContainerStepFunc is run3FSContainer factory func.
func NewRun3FSContainerStepFunc(setup *Run3FSContainerStepSetup) func() task.Step {
	return func() task.Step {
		return &run3FSContainerStep{
			imgName:        setup.ImgName,
			containerName:  setup.ContainerName,
			service:        setup.Service,
			serviceWorkDir: setup.WorkDir,
			extraVolumes:   setup.ExtraVolumes,
			useRdmaNetwork: setup.UseRdmaNetwork,
		}
	}
}

type rm3FSContainerStep struct {
	task.BaseStep

	containerName  string
	service        string
	serviceWorkDir string
}

func (s *rm3FSContainerStep) Execute(ctx context.Context) error {
	s.Logger.Infof("Removing %s container %s", s.service, s.containerName)
	_, err := s.Em.Docker.Rm(ctx, s.containerName, true)
	if err != nil {
		return errors.Trace(err)
	}
	s.Logger.Infof("Removed %s container %s successfully", s.service, s.containerName)

	configDir := getConfigDir(s.serviceWorkDir)
	_, err = s.Em.Runner.Exec(ctx, "rm", "-rf", configDir)
	if err != nil {
		return errors.Annotatef(err, "rm %s", configDir)
	}
	s.Logger.Infof("Removed %s container config dir %s", s.serviceWorkDir, configDir)

	logDir := path.Join(s.serviceWorkDir, "log")
	_, err = s.Em.Runner.Exec(ctx, "rm", "-rf", logDir)
	if err != nil {
		return errors.Annotatef(err, "rm %s", logDir)
	}
	s.Logger.Infof("Removed %s container log dir %s", s.serviceWorkDir, logDir)

	return nil
}

// NewRm3FSContainerStepFunc is rm3FSContainer factory func.
func NewRm3FSContainerStepFunc(containerName, service, serviceWorkDir string) func() task.Step {
	return func() task.Step {
		return &rm3FSContainerStep{
			containerName:  containerName,
			service:        service,
			serviceWorkDir: serviceWorkDir,
		}
	}
}

// GetMgmtdServerAddresses returns value of RuntimeMgmtdServerAddressesKey.
func GetMgmtdServerAddresses(r *task.Runtime) string {
	addrI, ok := r.Load(task.RuntimeMgmtdServerAddressesKey)
	if !ok {
		return ""
	}
	return addrI.(string)
}

type upload3FSMainConfigStep struct {
	task.BaseStep

	imgName        string
	containerName  string
	service        string
	serviceType    string
	serviceWorkDir string
}

func (s *upload3FSMainConfigStep) Execute(ctx context.Context) error {
	s.Logger.Infof("Upload %s main config", s.service)
	img, err := s.Runtime.Cfg.Images.GetImage(s.imgName)
	if err != nil {
		return errors.Trace(err)
	}
	args := &external.RunArgs{
		Image:       img,
		Name:        &s.containerName,
		HostNetwork: true,
		Privileged:  common.Pointer(true),
		Rm:          common.Pointer(true),
		Entrypoint:  common.Pointer("''"),
		Ulimits: map[string]string{
			"nofile": "1048576:1048576",
		},
		Command: []string{
			"/opt/3fs/bin/admin_cli",
			"-cfg", "/opt/3fs/etc/admin_cli.toml",
			"--config.mgmtd_client.mgmtd_server_addresses",
			fmt.Sprintf("'%s'", GetMgmtdServerAddresses(s.Runtime)),
			fmt.Sprintf("'set-config --type %s --file /opt/3fs/etc/%s.toml'",
				s.serviceType, s.service),
		},
		Volumes: []*external.VolumeArgs{
			{
				Source: getConfigDir(s.serviceWorkDir),
				Target: "/opt/3fs/etc/",
			},
			{
				Source: "/dev",
				Target: "/dev",
			},
		},
	}

	if err := s.GetErdmaSoPath(ctx); err != nil {
		return errors.Trace(err)
	}
	args.Volumes = append(args.Volumes, s.GetRdmaVolumes()...)
	_, err = s.Em.Docker.Run(ctx, args)
	if err != nil {
		return errors.Trace(err)
	}

	s.Logger.Infof("Service %s main config uploaded", s.service)
	return nil
}

// NewUpload3FSMainConfigStepFunc is upload3FSMainConfigStep factory func.
func NewUpload3FSMainConfigStepFunc(
	img, containerName, service, serviceWorkDir, serviceType string) func() task.Step {

	return func() task.Step {
		return &upload3FSMainConfigStep{
			imgName:        img,
			containerName:  containerName,
			service:        service,
			serviceWorkDir: serviceWorkDir,
			serviceType:    serviceType,
		}
	}
}

type remoteRunScriptStep struct {
	task.BaseStep

	workDir    string
	scriptName string
	script     []byte
	scriptArgs []string
}

func (s *remoteRunScriptStep) Execute(ctx context.Context) error {
	s.Logger.Infof("Start to run script %s on node", s.scriptName)
	localEm := s.Runtime.LocalEm
	tmpDir, err := localEm.FS.MkdirTemp(ctx, os.TempDir(), "remote-run-script")
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err := localEm.FS.RemoveAll(ctx, tmpDir); err != nil {
			s.Logger.Errorf("Failed to remove temporary directory %s: %v", tmpDir, err)
		}
	}()

	tmpScriptPath := path.Join(tmpDir, "/tmp_script.sh")
	if err = localEm.FS.WriteFile(tmpScriptPath, s.script, os.FileMode(0777)); err != nil {
		return errors.Trace(err)
	}

	if err = s.Em.FS.MkdirAll(ctx, s.workDir); err != nil {
		return errors.Trace(err)
	}
	remoteFile, err := s.Em.FS.MkTempFile(ctx, os.TempDir())
	if err != nil {
		return errors.Annotate(err, "make temp file")
	}

	defer func() {
		if _, err := s.Em.Runner.Exec(ctx, "rm", "-f", remoteFile); err != nil {
			s.Logger.Errorf("Failed to remove remote file %s: %v", remoteFile, err)
		}
	}()

	s.Logger.Infof("Scp %s to %s", tmpScriptPath, remoteFile)
	if err = s.Em.Runner.Scp(ctx, tmpScriptPath, remoteFile); err != nil {
		return errors.Trace(err)
	}

	s.Logger.Infof("Run %s with %v", s.scriptName, s.scriptArgs)
	out, err := s.Em.Runner.Exec(ctx, "bash", append([]string{remoteFile}, s.scriptArgs...)...)
	if err != nil {
		return errors.Trace(err)
	}
	s.Logger.Debugf("Run %s output: %s", s.scriptName, out)

	s.Logger.Infof("Run %s success", s.scriptName)

	return nil
}

// NewRemoteRunScriptStepFunc is remoteRunScriptStep factory func.
func NewRemoteRunScriptStepFunc(
	workDir, scriptName string, script []byte, scriptArgs []string) func() task.Step {

	return func() task.Step {
		return &remoteRunScriptStep{
			workDir:    workDir,
			scriptName: scriptName,
			script:     script,
			scriptArgs: scriptArgs,
		}
	}
}
