package external

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/open3fs/m3fs/pkg/errors"
)

// RunInterface is the interface for running command.
type RunInterface interface {
	Run(ctx context.Context, command string, args ...string) (*bytes.Buffer, error)
}

// RemoteRunner implements RunInterface by running command on a remote host.
type RemoteRunner struct {
	mu         sync.Mutex
	sshClient  *ssh.Client
	sftpClient *sftp.Client
}

// Run is used for running a command.
func (r *RemoteRunner) Run(ctx context.Context, command string, args ...string) (
	*bytes.Buffer, error) {

	session, err := r.newSession()
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer session.Close()

	var output bytes.Buffer
	session.Stdout = &output
	if err := session.Run("ls -l /tmp"); err != nil {
		return nil, errors.Trace(err)
	}
	return &output, nil
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
	r.sshClient.Close()
	r.sshClient = nil
}

// RemoteRunnerCfg defines configurations of a remote runner.
type RemoteRunnerCfg struct {
	Username   string
	Password   *string
	TargetHost string
	TargetPort int
	PrivateKey *string
	Timeout    time.Duration
}

// NewRemoteRunner creates a remote runner.
func NewRemoteRunner(cfg *RemoteRunnerCfg) (*RemoteRunner, error) {
	authMethods := make([]ssh.AuthMethod, 0)
	if cfg.Password != nil {
		authMethods = append(authMethods, ssh.Password(*cfg.Password))
	}
	if cfg.PrivateKey != nil {
		signer, parseErr := ssh.ParsePrivateKey([]byte(*cfg.PrivateKey))
		if parseErr != nil {
			return nil, errors.Annotatef(parseErr, "parse private key")
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
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
		sshClient:  sshClient,
		sftpClient: sftpClient,
	}

	return runner, nil
}
