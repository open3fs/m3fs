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

package network

import (
	"bytes"
	"context"
	"embed"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/task"
)

var (
	//go:embed templates/*.tmpl
	templatesFs embed.FS

	// SetupRxeNetworkTmpl is the template content of setup rxe network script
	SetupRxeNetworkTmpl []byte
	// SetupErdmaNetworkTmpl is the template content of setup erdma network script
	SetupErdmaNetworkTmpl []byte
	// SetupServiceTmpl is the template content of setup network service
	SetupServiceTmpl []byte
)

func init() {
	var err error
	SetupRxeNetworkTmpl, err = templatesFs.ReadFile("templates/setup-rxe-network.tmpl")
	if err != nil {
		panic(err)
	}

	SetupErdmaNetworkTmpl, err = templatesFs.ReadFile("templates/setup-erdma-network.tmpl")
	if err != nil {
		panic(err)
	}

	SetupServiceTmpl, err = templatesFs.ReadFile("templates/setup-3fs-network.service.tmpl")
	if err != nil {
		panic(err)
	}
}

const (
	genIbdev2netdevScript = `#!/bin/bash

# emulate mlnx util ibdev2netdev
# format: ibdev port xx ==> netdev (Up/Down)
# example: mlx5_0 port 1 ==> eth0 (Up)

if [ -z "$1" ]; then
    echo "Usage: $0 <targetDir>"
    exit 1
fi

targetDir=$1
targetScript="$targetDir/ibdev2netdev"
if [ -d "$targetScript" ]; then
	rm -fr "$targetScript"
fi

cat > "$targetScript" <<EOF
#!/bin/bash
EOF

rdma link | awk '{print $2,$4,$8}' | while read ibInfo state netdev
do
	ibdev=$(echo $ibInfo | cut -d/ -f1)
	ibport=$(echo $ibInfo | cut -d/ -f2)

    if [ "$state" = "ACTIVE" ]; then
        state=Up
    else
        state=Down
    fi
    if [ -n "$netdev" ]; then
		cat <<EOF >> $targetScript
echo "$ibdev port $ibport ==> $netdev ($state)"
EOF
    fi
done

chmod +x $targetScript
`
)

var rdmaPackages = []string{
	"iproute2",
	"libibverbs1",
	"ibverbs-utils",
	"librdmacm1",
	"libibumad3",
	"ibverbs-providers",
	"rdma-core",
	"rdmacm-utils",
	"perftest",
}

type genIbdev2netdevScriptStep struct {
	task.BaseStep
}

func (s *genIbdev2netdevScriptStep) Execute(ctx context.Context) error {
	s.Logger.Debugf("Generating ibdev2netdev script for %s", s.Node.Host)
	localEm := s.Runtime.LocalEm
	tmpDir, err := localEm.FS.MkdirTemp(ctx, os.TempDir(), "m3fs-prepare-network")
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err := localEm.FS.RemoveAll(ctx, tmpDir); err != nil {
			s.Logger.Errorf("Failed to remove temporary directory %s: %v", tmpDir, err)
		}
	}()
	scriptLocalPath := path.Join(tmpDir, "gen-ibdev2netdev")
	err = localEm.FS.WriteFile(scriptLocalPath, []byte(genIbdev2netdevScript), 0755)
	if err != nil {
		return errors.Trace(err)
	}

	binDir := path.Join(s.Runtime.Cfg.WorkDir, "bin")
	err = s.Em.FS.MkdirAll(ctx, binDir)
	if err != nil {
		return errors.Annotatef(err, "mkdir %s", binDir)
	}
	remoteGenScriptPath := "/tmp/gen-ibdev2netdev"
	if err := s.Em.Runner.Scp(ctx, scriptLocalPath, remoteGenScriptPath); err != nil {
		return errors.Annotatef(err, "scp %s", scriptLocalPath)
	}
	_, err = s.Em.Runner.Exec(ctx, "bash", remoteGenScriptPath, binDir)
	if err != nil {
		return errors.Annotatef(err, "bash %s %s", remoteGenScriptPath, binDir)
	}
	return nil
}

type installRdmaPackageStep struct {
	task.BaseStep
}

func (s *installRdmaPackageStep) Execute(ctx context.Context) error {
	s.Logger.Debugf("Installing rdma related packages for %s", s.Node.Host)

	_, err := s.Em.Runner.Exec(ctx, "apt", "install", "-y", strings.Join(rdmaPackages, " "))
	if err != nil {
		return errors.Annotatef(err, "install rdma related packages")
	}
	return nil
}

type setupNetworkBaseStep struct {
	task.BaseStep
}

func (s *setupNetworkBaseStep) setupNetwork(ctx context.Context, scriptData []byte) error {
	scriptPath := path.Join(s.Runtime.WorkDir, "bin", "setup-network")
	tmpl, err := template.New("setup-3fs-network.service.tmpl").Parse(string(SetupServiceTmpl))
	if err != nil {
		return errors.Trace(err)
	}
	data := map[string]any{
		"ScriptPath": scriptPath,
	}
	buf := bytes.NewBuffer(nil)
	if err = tmpl.Execute(buf, data); err != nil {
		return errors.Trace(err)
	}
	s.Logger.Infof("Creating setup-network script and setup-3fs-network.service service...")
	err = s.CreateScriptAndService(ctx, "setup-network", "setup-3fs-network.service",
		scriptData, buf.Bytes())
	if err != nil {
		return errors.Trace(err)
	}

	_, err = s.Em.Runner.Exec(ctx, scriptPath)
	if err != nil {
		return errors.Annotatef(err, "execute %s", scriptPath)
	}

	return nil
}

type setupRxeStep struct {
	setupNetworkBaseStep
}

func (s *setupRxeStep) Execute(ctx context.Context) error {
	s.Logger.Infof("Setup rxe environment for node %s", s.Node.Host)

	return errors.Trace(s.setupNetwork(ctx, SetupRxeNetworkTmpl))
}

type setupErdmaStep struct {
	setupNetworkBaseStep
}

func (s *setupErdmaStep) Execute(ctx context.Context) error {
	s.Logger.Infof("Setup erdma environment for node %s", s.Node.Host)

	return errors.Trace(s.setupNetwork(ctx, SetupErdmaNetworkTmpl))
}

type deleteSetupNetworkServiceStep struct {
	task.BaseStep
}

func (s *deleteSetupNetworkServiceStep) Execute(ctx context.Context) error {
	return errors.Trace(s.DeleteService(ctx, "setup-3fs-network.service"))
}

type deleteIbdev2netdevScriptStep struct {
	task.BaseStep
}

func (s *deleteIbdev2netdevScriptStep) Execute(ctx context.Context) error {
	s.Logger.Debugf("Deleting ibdev2netdev script for %s", s.Node.Host)
	binDir := path.Join(s.Runtime.Cfg.WorkDir, "bin")
	_, err := s.Em.Runner.Exec(ctx, "rm", "-f", path.Join(binDir, "ibdev2netdev"))
	if err != nil {
		return errors.Annotatef(err, "rm %s", path.Join(binDir, "ibdev2netdev"))
	}
	return nil
}
