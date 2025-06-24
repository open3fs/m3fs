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

package external

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/log"
)

// RunnerInterface is the interface for running command.
type RunnerInterface interface {
	NonSudoExec(ctx context.Context, command string, args ...string) (string, error)
	Exec(ctx context.Context, command string, args ...string) (string, error)

	Scp(ctx context.Context, local, remote string) error
	Stat(path string) (os.FileInfo, error)
}

// RemoteRunner implements RunInterface by running command on a remote host.
type RemoteRunner struct {
	mu         sync.Mutex
	log        log.Interface
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	user       string
	password   string
}

func (r *RemoteRunner) exec(cmd string) (string, error) {
	session, err := r.newSession()
	if err != nil {
		return "", errors.Trace(err)
	}
	defer func() {
		if err := session.Close(); err != nil && !errors.Is(err, io.EOF) {
			r.log.Warnf("Failed to close session: %v", err)
		}
	}()

	in, err := session.StdinPipe()
	if err != nil {
		return "", errors.Annotate(err, "get session stdinpipe")
	}
	out, err := session.StdoutPipe()
	if err != nil {
		return "", errors.Annotate(err, "get session stdoutpipe")
	}
	errOut, err := session.StderrPipe()
	if err != nil {
		return "", errors.Annotate(err, "get session stderrpipe")
	}

	r.log.Debugf("Run command: %s", cmd)
	err = session.Start(cmd)
	if err != nil {
		return "", errors.Trace(err)
	}

	var (
		output    []byte
		line      = ""
		outReader = bufio.NewReader(out)
	)

	requirePasswordPrefix := fmt.Sprintf("[sudo] password for %s: ", r.user)
	for {
		b, err := outReader.ReadByte()
		if err != nil {
			if err != io.EOF {
				r.log.Debugf("Failed to read data from stdout: %s", err)
			}
			break
		}

		output = append(output, b)
		if b == byte('\n') {
			line = ""
			continue
		}

		line += string(b)

		if (strings.HasPrefix(line, requirePasswordPrefix) || strings.HasPrefix(line, "Password")) &&
			strings.HasSuffix(line, ": ") {

			line = ""
			_, err = in.Write([]byte(r.password + "\n"))
			if err != nil {
				r.log.Debugf("Failed to input sudo password: %s", err)
				break
			}
		}
	}

	errReader := bufio.NewReader(errOut)
	errBytes, err := io.ReadAll(errReader)
	if err != nil {
		r.log.Warnf("Failed to read error output of `%s`: %v", cmd, err)
	}
	err = session.Wait()
	outStr := strings.ReplaceAll(string(output), requirePasswordPrefix, "")
	r.log.Debugf("Output of `%s`: %s", cmd, outStr)
	r.log.Debugf("Error output of `%s`: %s", cmd, string(errBytes))
	if err != nil {
		return "", errors.Annotatef(err, "run `%s` failed", cmd)
	}

	return outStr, nil
}

// NonSudoExec executes a command.
func (r *RemoteRunner) NonSudoExec(ctx context.Context, command string, args ...string) (string, error) {
	cmdStr := strings.Join(append([]string{command}, args...), " ")
	out, err := r.exec(cmdStr)
	if err != nil {
		return "", errors.Trace(err)
	}

	return out, nil
}

// Exec executes a command with sudo.
func (r *RemoteRunner) Exec(ctx context.Context, command string, args ...string) (string, error) {
	cmdStr := strings.Join(append([]string{command}, args...), " ")
	out, err := r.exec(fmt.Sprintf("sudo %s", cmdStr))
	if err != nil {
		return "", errors.Trace(err)
	}

	return out, nil
}

func (r *RemoteRunner) newSession() (*ssh.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sshClient == nil {
		return nil, errors.New("SSH Client is not found")
	}

	session, err := r.sshClient.NewSession()
	if err != nil {
		return nil, errors.Trace(err)
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err = session.RequestPty("xterm", 100, 50, modes); err != nil {
		return nil, errors.Trace(err)
	}
	if err = session.Setenv("LANG", "en_US.UTF-8"); err != nil {
		return nil, errors.Trace(err)
	}
	return session, nil
}

// Close closes the runner.
func (r *RemoteRunner) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sshClient == nil {
		return
	}
	if err := r.sshClient.Close(); err != nil {
		r.log.Warnf("Failed to close SSH client: %+v", err)
	}
	r.sshClient = nil
}

// Scp copy local file or dir to remote host.
func (r *RemoteRunner) Scp(ctx context.Context, local, remote string) error {
	r.log.Debugf("Scp %s on the local node to %s on the remote node", local, remote)
	f, err := os.Stat(local)
	if err != nil {
		return errors.Trace(err)
	}
	if !f.IsDir() {
		if err := r.copyFileToRemote(local, remote); err != nil {
			if e, ok := errors.Cause(err).(*sftp.StatusError); ok {
				r.log.Errorf("Failed to copy %s to %s: %v", local, remote, e)
			}
			return errors.Trace(err)
		}
		return nil
	}
	if err := r.copyDirToRemote(local, remote); err != nil {
		if e, ok := errors.Cause(err).(*sftp.StatusError); ok {
			r.log.Errorf("Failed to copy %s to %s: %v", local, remote, e)
		}
		return errors.Trace(err)
	}
	return nil
}

func (r *RemoteRunner) copyFileToRemote(local, remote string) error {
	localFile, err := os.Open(local)
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err := localFile.Close(); err != nil {
			r.log.Warnf("Failed to close local file: %+v", err)
		}
	}()
	remoteFile, err := r.sftpClient.Create(remote)
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err := remoteFile.Close(); err != nil {
			r.log.Warnf("Failed to close remote file: %+v", err)
		}
	}()
	_, err = io.Copy(remoteFile, localFile)
	return errors.Trace(err)
}

func (r *RemoteRunner) copyDirToRemote(local, remote string) error {
	if err := r.sftpClient.Mkdir(remote); err != nil && !os.IsExist(err) {
		return errors.Trace(err)
	}

	return filepath.Walk(local, func(localFile string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Trace(err)
		}
		relPath, _ := filepath.Rel(local, localFile)
		remoteFile := filepath.Join(remote, relPath)
		if info.IsDir() {
			err := r.sftpClient.Mkdir(remoteFile)
			if err != nil && os.IsExist(err) {
				return errors.Trace(err)
			}
			return nil
		}
		if err = r.copyFileToRemote(localFile, remoteFile); err != nil {
			return errors.Trace(err)
		}
		return nil
	})
}

// Stat run stat on path
func (r *RemoteRunner) Stat(path string) (os.FileInfo, error) {
	info, err := r.sftpClient.Stat(path)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return info, nil
}

// RemoteRunnerCfg defines configurations of a remote runner.
type RemoteRunnerCfg struct {
	Username   string
	Password   *string
	TargetHost string
	TargetPort int
	PrivateKey *string
	Logger     log.Interface
	Timeout    time.Duration
}

// NewRemoteRunner creates a remote runner.
func NewRemoteRunner(cfg *RemoteRunnerCfg) (*RemoteRunner, error) {
	authMethods := make([]ssh.AuthMethod, 0)
	privateKeyPath := filepath.Join(os.Getenv("HOME"), ".ssh/id_rsa")
	if _, err := os.Stat(privateKeyPath); err == nil {
		privateKey, err := os.ReadFile(privateKeyPath)
		if err == nil {
			signer, parseErr := ssh.ParsePrivateKey(privateKey)
			if parseErr != nil {
				return nil, errors.Annotatef(parseErr, "parse private key")
			}
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}
	if cfg.Password != nil {
		authMethods = append(authMethods, ssh.Password(*cfg.Password))
	}
	sshConfig := &ssh.ClientConfig{
		User:            cfg.Username,
		Timeout:         cfg.Timeout,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	endpoint := net.JoinHostPort(cfg.TargetHost, strconv.Itoa(cfg.TargetPort))
	sshClient, err := ssh.Dial("tcp", endpoint, sshConfig)
	if err != nil {
		return nil, errors.Annotatef(err, "establish connection to %s", endpoint)
	}
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, errors.Annotatef(err, "new sftp client")
	}
	runner := &RemoteRunner{
		user:       cfg.Username,
		log:        cfg.Logger,
		sshClient:  sshClient,
		sftpClient: sftpClient,
	}
	if cfg.Password != nil {
		runner.password = *cfg.Password
	}

	return runner, nil
}
