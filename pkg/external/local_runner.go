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
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/log"
)

// LocalRunner implements RunInterface by running command on local host.
type LocalRunner struct {
	logger         log.Interface
	maxExitTimeout time.Duration
	user           string
	password       string
}

// NonSudoExec executes a command.
func (r *LocalRunner) NonSudoExec(ctx context.Context, command string, args ...string) (string, error) {
	checkErr := func(err error, errOut string) RunError {
		switch err {
		case context.Canceled:
			return NewRunError(int(syscall.ECANCELED), "process canceled")
		case context.DeadlineExceeded:
			return NewRunError(int(syscall.ETIMEDOUT), "process timeout")
		default:
			if err == nil {
				return nil
			}

			if msg, ok := err.(*exec.ExitError); ok {
				return &runErrorImpl{
					code: msg.Sys().(syscall.WaitStatus).ExitStatus(),
					msg:  fmt.Sprintf("%s\n%s", err, errOut),
				}
			}
			return &runErrorImpl{
				code: -1,
				msg:  fmt.Sprintf("%s\n%s", err, errOut),
			}
		}
	}

	cmdStr := fmt.Sprintf("%s %s", command, strings.Join(args, " "))
	r.logger.Debugf("Run command: %s", cmdStr)
	out := new(bytes.Buffer)
	cmd := exec.Command(command, args...)
	in, err := cmd.StdinPipe()
	if err != nil {
		return "", errors.Annotate(err, "get cmd stdinpipe")
	}
	errOut, err := cmd.StderrPipe()
	if err != nil {
		return "", errors.Annotate(err, "get cmd stderrpipe")
	}
	cmd.Stdout = out
	errOutStr, err := r.runCtx(ctx, cmd, in, errOut)
	if err != nil {
		r.logger.Debugf("Output of %s: %s", cmdStr, out.String())
		return "", checkErr(err, errOutStr)
	}

	if _, err = out.WriteString(errOutStr); err != nil {
		return "", errors.Annotate(err, "append stderr output")
	}
	outStr := out.String()
	r.logger.Debugf("Output of %s: %s", cmdStr, outStr)

	return outStr, nil
}

// Exec executes a command with sudo
func (r *LocalRunner) Exec(ctx context.Context, cmd string, args ...string) (string, error) {
	return r.NonSudoExec(ctx, "sudo",
		[]string{
			"-S",
			"/bin/bash",
			"-c",
			strings.Join(append([]string{cmd}, args...), " "),
		}...)
}

// Scp copy local file or dir to remote host.
func (r *LocalRunner) Scp(ctx context.Context, local, remote string) error {
	_, err := r.Exec(ctx, "cp", "-r", local, remote)
	return errors.Trace(err)
}

// Wait is an essential part of exec.Cmd which must have been started by Start,
// even though cmd is killed.
//
// It is necessary to wait for the command to exit and wait for any copying
// to stdin or copying from stdout or stderr to complete after `Kill`, because
// Kill only causes the process to exit immediately, but does not wait until
// the process has actually exited.
//
// Note: if sub process is in state D (waiting for IO), the goroutine will leak.
// There is no way to guarantee neither hanging nor leaking resources
func (r *LocalRunner) runCtx(ctx context.Context, cmd *exec.Cmd,
	in io.WriteCloser, errOut io.ReadCloser) (errOutStr string, err error) {

	startTime := time.Now()
	maxExitTimeout := r.maxExitTimeout

	if err = cmd.Start(); err != nil {
		return "", err
	}

	var (
		output       []byte
		line         = ""
		errOutReader = bufio.NewReader(errOut)
	)

	requirePasswordPrefix := "[sudo] password for "
	if r.user != "" {
		requirePasswordPrefix = fmt.Sprintf("[sudo] password for %s: ", r.user)
	}
	for {
		b, err := errOutReader.ReadByte()
		if err != nil {
			if err != io.EOF {
				r.logger.Debugf("Failed to read data from stdout: %s", err)
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
				r.logger.Debugf("Failed to input sudo password: %s", err)
				break
			}
		}
	}
	errOutStr = strings.ReplaceAll(string(output), requirePasswordPrefix, "")

	done := make(chan error)
	go func() {
		done <- cmd.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		d := time.Since(startTime)
		if err = cmd.Process.Kill(); err != nil {
			return errOutStr, err
		}
		select {
		case <-done:
		case <-time.After(maxExitTimeout):
			r.logger.Warnf("Wait for command to exit timeout: %s", maxExitTimeout)
			return errOutStr, errors.Errorf("wait process to exit timeout after %s", maxExitTimeout)
		}
		if b, ok := cmd.Stdout.(*bytes.Buffer); ok {
			r.logger.Warnf("Process was killed after %v: %s %s\nstdout: %v\nstderr: %v",
				d.Round(100*time.Millisecond), cmd.Path, cmd.Args, b.String(), cmd.Stderr)
		} else {
			// Reduce time accuracy, avoid frequent log changes that affect logger rate limit
			r.logger.Warnf("Process was killed after %v: %s %s\nstdout: %v\nstderr: %v",
				d.Round(100*time.Millisecond), cmd.Path, cmd.Args, cmd.Stdout, cmd.Stderr)
		}
		return errOutStr, ctx.Err()
	case err = <-done:
		return errOutStr, err
	}
}

// RunError is the wrapper of os.exec error, it export error code
type RunError interface {
	ExitCode() int
	Error() string
	ExitCodeEquals(syscall.Errno) bool
	ExitCodeIn(...syscall.Errno) bool
}

// NewRunError is used to get the RunError
func NewRunError(code int, msg string) (err RunError) {
	return &runErrorImpl{
		code: code,
		msg:  msg,
	}
}

type runErrorImpl struct {
	code int
	msg  string
}

func (e runErrorImpl) Error() string {
	return e.msg
}

func (e runErrorImpl) ExitCode() int {
	return e.code
}

func (e runErrorImpl) ExitCodeEquals(errno syscall.Errno) bool {
	return e.code == int(errno)
}

func (e runErrorImpl) ExitCodeIn(errnos ...syscall.Errno) bool {
	for _, errno := range errnos {
		if e.code == int(errno) {
			return true
		}
	}
	return false
}

// LocalRunnerCfg defines configurations of a local runner.
type LocalRunnerCfg struct {
	Logger         log.Interface
	MaxExitTimeout *time.Duration
	User           string
	Password       string
}

// NewLocalRunner creates a local runner.
func NewLocalRunner(cfg *LocalRunnerCfg) *LocalRunner {
	maxExitTimeout := time.Minute * 10
	if cfg.MaxExitTimeout != nil {
		maxExitTimeout = *cfg.MaxExitTimeout
	}

	return &LocalRunner{
		logger:         cfg.Logger,
		maxExitTimeout: maxExitTimeout,
		user:           cfg.User,
		password:       cfg.Password,
	}
}
