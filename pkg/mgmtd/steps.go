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
	"bytes"
	"context"
	"embed"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/task"
	"github.com/open3fs/m3fs/pkg/task/steps"
)

var (
	//go:embed templates/*.tmpl
	templatesFs embed.FS

	// MgmtdMainAppTomlTmpl is the template content of mgmtd_main_app.toml
	MgmtdMainAppTomlTmpl []byte
	// MgmtdMainLauncherTomlTmpl is the template content of mgmtd_main_launcher.toml
	MgmtdMainLauncherTomlTmpl []byte
	// MgmtdMainTomlTmpl is the template content of mgmtd_main.toml
	MgmtdMainTomlTmpl []byte
	// AdminCliTomlTmpl is the template content of admin_cli.toml
	AdminCliTomlTmpl []byte
	// AdminCliShellTmpl is the template content of admin_cli.sh
	AdminCliShellTmpl []byte
)

func init() {
	var err error
	MgmtdMainAppTomlTmpl, err = templatesFs.ReadFile("templates/mgmtd_main_app.toml.tmpl")
	if err != nil {
		panic(err)
	}

	MgmtdMainLauncherTomlTmpl, err = templatesFs.ReadFile("templates/mgmtd_main_launcher.toml.tmpl")
	if err != nil {
		panic(err)
	}

	MgmtdMainTomlTmpl, err = templatesFs.ReadFile("templates/mgmtd_main.toml.tmpl")
	if err != nil {
		panic(err)
	}

	AdminCliTomlTmpl, err = templatesFs.ReadFile("templates/admin_cli.toml.tmpl")
	if err != nil {
		panic(err)
	}

	AdminCliShellTmpl, err = templatesFs.ReadFile("templates/admin_cli.sh.tmpl")
	if err != nil {
		panic(err)
	}
}

type genAdminCliConfigStep struct {
	task.BaseStep
}

func (s *genAdminCliConfigStep) Execute(ctx context.Context) error {
	mgmtdServerAddresses := make([]string, len(s.Runtime.Services.Mgmtd.Nodes))
	port := strconv.Itoa(s.Runtime.Services.Mgmtd.RDMAListenPort)
	for i, nodeName := range s.Runtime.Services.Mgmtd.Nodes {
		node := s.Runtime.Nodes[nodeName]
		mgmtdServerAddresses[i] = fmt.Sprintf(`"%s://%s"`,
			s.Runtime.MgmtdProtocol, net.JoinHostPort(node.Host, port))
	}
	mgmtdServerAddressesStr := fmt.Sprintf("[%s]", strings.Join(mgmtdServerAddresses, ","))
	s.Runtime.Store(task.RuntimeMgmtdServerAddressesKey, mgmtdServerAddressesStr)

	adminCliData := map[string]any{
		"ClusterID":            s.Runtime.Cfg.Name,
		"MgmtdServerAddresses": mgmtdServerAddressesStr,
	}
	s.Logger.Debugf("Admin cli config template data: %v", adminCliData)
	t, err := template.New("admin_cli.toml").Parse(string(AdminCliTomlTmpl))
	if err != nil {
		return errors.Annotatef(err, "parse template of admin_cli.toml.tmpl")
	}
	data := new(bytes.Buffer)
	err = t.Execute(data, adminCliData)
	if err != nil {
		return errors.Annotate(err, "execute template of admin_cli.toml.tmpl")
	}
	s.Runtime.Store(task.RuntimeAdminCliTomlKey, data.Bytes())

	return nil
}

type initClusterStep struct {
	task.BaseStep
}

func (s *initClusterStep) Execute(ctx context.Context) error {
	mgmtd := s.Runtime.Services.Mgmtd
	img, err := s.Runtime.Cfg.Images.GetImage(config.ImageName3FS)
	if err != nil {
		return errors.Trace(err)
	}

	workDir := getServiceWorkDir(s.Runtime.WorkDir)
	logDir := path.Join(workDir, "log")
	err = s.Em.FS.MkdirAll(ctx, logDir)
	if err != nil {
		return errors.Annotatef(err, "mkdir %s", logDir)
	}
	args := &external.RunArgs{
		Image:      img,
		Name:       &mgmtd.ContainerName,
		Entrypoint: common.Pointer("''"),
		Rm:         common.Pointer(true),
		Command: []string{
			"/opt/3fs/bin/admin_cli",
			"-cfg", "/opt/3fs/etc/admin_cli.toml",
			fmt.Sprintf("'init-cluster --mgmtd /opt/3fs/etc/mgmtd_main.toml 1 %d %d'",
				mgmtd.ChunkSize, mgmtd.StripeSize)},
		HostNetwork: true,
		Volumes: []*external.VolumeArgs{
			{
				Source: "/dev",
				Target: "/dev",
			},
			{
				Source: path.Join(workDir, "config.d"),
				Target: "/opt/3fs/etc",
			},
			{
				Source: logDir,
				Target: "/var/log/3fs",
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

	s.Logger.Infof("Cluster initialization success")
	return nil
}

type genAdminCliShellStep struct {
	task.BaseStep
}

func (s *genAdminCliShellStep) Execute(ctx context.Context) error {
	tempDir, err := s.Runtime.LocalEm.FS.MkdirTemp(ctx, os.TempDir(), "3fs-mgmtd")
	if err != nil {
		return errors.Trace(err)
	}

	t, err := template.New("admin_cli.sh").Parse(string(AdminCliShellTmpl))
	if err != nil {
		return errors.Annotatef(err, "parse template of admin_cli.sh.tmpl")
	}
	data := new(bytes.Buffer)
	err = t.Execute(data, nil)
	if err != nil {
		return errors.Annotate(err, "execute template of admin_cli.sh.tmpl")
	}
	srcShellPath := filepath.Join(tempDir, "admin_cli.sh")
	if err = s.Runtime.LocalEm.FS.WriteFile(srcShellPath, data.Bytes(), 0777); err != nil {
		return errors.Trace(err)
	}
	dstShellPath := filepath.Join(s.Runtime.WorkDir, "admin_cli.sh")
	if err = s.Em.Runner.Scp(ctx, srcShellPath, dstShellPath); err != nil {
		return errors.Trace(err)
	}
	if err = s.Runtime.LocalEm.FS.RemoveAll(ctx, tempDir); err != nil {
		return errors.Trace(err)
	}

	return nil
}

type initUserAndChainStep struct {
	task.BaseStep
}

func (s *initUserAndChainStep) Execute(ctx context.Context) error {
	token, err := s.initUser(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	if err = s.initChainFiles(ctx); err != nil {
		return errors.Trace(err)
	}
	if err = s.uploadChainFiles(ctx, token); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (s *initUserAndChainStep) initUser(ctx context.Context) (token string, err error) {
	addr := steps.GetMgmtdServerAddresses(s.Runtime)
	output, err := s.Em.Docker.Exec(ctx, s.Runtime.Services.Mgmtd.ContainerName,
		"/opt/3fs/bin/admin_cli",
		"-cfg", "/opt/3fs/etc/admin_cli.toml",
		"--config.mgmtd_client.mgmtd_server_addresses", fmt.Sprintf(`'%s'`, addr),
		`"user-add --root --admin 0 root"`,
	)
	if err != nil {
		return "", errors.Annotate(err, "add user")
	}
	// Sample output:
	// Uid                0
	// Name               root
	// Token              AAA8WCoB8QAt8bFw2wBupzjA(Expired at N/A)
	// IsRootUser         true
	// IsAdmin            true
	// Gid                0
	// SupplementaryGids
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "Token") {
			continue
		}
		parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "Token")), "(")
		if len(parts) != 2 {
			break
		}
		token = parts[0]
		break
	}
	if token == "" {
		return "", errors.Errorf("Unexpected output of user-add command: %s", output)
	}

	_, err = s.Em.Docker.Exec(ctx, s.Runtime.Services.Mgmtd.ContainerName,
		"bash", "-c",
		fmt.Sprintf(`"echo %s > /opt/3fs/etc/token.txt"`, token),
	)
	if err != nil {
		return "", errors.Trace(err)
	}
	s.Runtime.Store(task.RuntimeUserTokenKey, token)
	return token, nil
}

func (s *initUserAndChainStep) initChainFiles(ctx context.Context) error {
	output, err := s.Em.Docker.Exec(ctx, s.Runtime.Services.Mgmtd.ContainerName,
		"python3", "/opt/3fs/data_placement/src/model/data_placement.py",
		"-ql", "-relax", "-type", "CR",
		"--num_nodes", strconv.Itoa(len(s.Runtime.Services.Storage.Nodes)),
		"--replication_factor", strconv.Itoa(s.Runtime.Services.Storage.ReplicationFactor),
		"--min_targets_per_disk", strconv.Itoa(s.Runtime.Services.Storage.TargetNumPerDisk),
	)
	if err != nil {
		return errors.Annotatef(err, "run data_placement.py")
	}
	var dataPlacementDir string
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "saved solution to: ") {
			continue
		}
		parts := strings.Split(line, " ")
		dataPlacementDir = strings.TrimSpace(parts[len(parts)-1])
	}
	if dataPlacementDir == "" {
		return errors.Errorf("Unexpected output of data_placement.py: %s", output)
	}

	_, err = s.Em.Docker.Exec(ctx, s.Runtime.Services.Mgmtd.ContainerName,
		"python3", "/opt/3fs/data_placement/src/setup/gen_chain_table.py",
		"--chain_table_type", "CR",
		"--node_id_begin", "10001",
		"--node_id_end", strconv.Itoa(10000+len(s.Runtime.Services.Storage.Nodes)),
		"--num_disks_per_node", strconv.Itoa(s.Runtime.Services.Storage.DiskNumPerNode),
		"--num_targets_per_disk", strconv.Itoa(s.Runtime.Services.Storage.TargetNumPerDisk),
		"--target_id_prefix", strconv.Itoa(s.Runtime.Services.Storage.TargetIDPrefix),
		"--chain_id_prefix", strconv.Itoa(s.Runtime.Services.Storage.ChainIDPrefix),
		"--incidence_matrix_path", fmt.Sprintf("%s/incidence_matrix.pickle", dataPlacementDir),
	)
	if err != nil {
		return errors.Annotatef(err, "run gen_chain_table.py")
	}

	return nil
}

func (s *initUserAndChainStep) uploadChainFiles(ctx context.Context, token string) error {
	addr := steps.GetMgmtdServerAddresses(s.Runtime)
	escapedAddr := strings.Replace(addr, `"`, `\"`, -1)
	_, err := s.Em.Docker.Exec(ctx, s.Runtime.Services.Mgmtd.ContainerName,
		"bash", "-c",
		fmt.Sprintf(
			`"/opt/3fs/bin/admin_cli --cfg /opt/3fs/etc/admin_cli.toml `+
				`--config.mgmtd_client.mgmtd_server_addresses '%s' `+
				`--config.user_info.token %s < output/create_target_cmd.txt"`,
			escapedAddr, token),
	)
	if err != nil {
		return errors.Annotatef(err, "create targets")
	}
	_, err = s.Em.Docker.Exec(ctx, s.Runtime.Services.Mgmtd.ContainerName,
		"/opt/3fs/bin/admin_cli",
		"--cfg", "/opt/3fs/etc/admin_cli.toml",
		"--config.mgmtd_client.mgmtd_server_addresses", fmt.Sprintf(`'%s'`, addr),
		"--config.user_info.token", token,
		`"upload-chains output/generated_chains.csv"`,
	)
	if err != nil {
		return errors.Annotatef(err, "upload-chains output/generated_chains.csv")
	}
	_, err = s.Em.Docker.Exec(ctx, s.Runtime.Services.Mgmtd.ContainerName,
		"/opt/3fs/bin/admin_cli",
		"--cfg", "/opt/3fs/etc/admin_cli.toml",
		"--config.mgmtd_client.mgmtd_server_addresses", fmt.Sprintf(`'%s'`, addr),
		"--config.user_info.token", token,
		`"upload-chain-table --desc stage 1 output/generated_chain_table.csv"`,
	)
	if err != nil {
		return errors.Annotatef(err, "upload-chain-table output/generated_chain_table.csv")
	}

	return nil
}
