package external

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/open3fs/m3fs/pkg/errors"
)

// NewProcessRegexp compile process name with possible prefix paths
func NewProcessRegexp(format string, v ...any) *regexp.Regexp {
	if strings.HasPrefix(format, "/") {
		return regexp.MustCompile(fmt.Sprintf("^"+format+"$", v...))
	}
	return regexp.MustCompile(fmt.Sprintf("^(/usr/local/bin/|/usr/bin/|/bin/|/usr/sbin/|/sbin/|)"+format+"$", v...))
}

// NewProcessNameRegexp init regexp with process name
func NewProcessNameRegexp(name string) *regexp.Regexp {
	return NewProcessRegexp("%s(\\s+.*|)", name)
}

func realRun(ctx context.Context, command string, args ...string) (*bytes.Buffer, RunError) {

	checkErr := func(err error, errOut *bytes.Buffer) RunError {
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

	logrus.Debugf("Run command: %s %s", command, strings.Join(args, " "))
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	cmd := exec.Command(command, args...)
	cmd.Stdout = out
	cmd.Stderr = errOut
	err := runCtx(ctx, cmd)
	if err != nil {
		return out, checkErr(err, errOut)
	}

	return out, nil
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
func runCtx(ctx context.Context, cmd *exec.Cmd) (err error) {
	startTime := time.Now()
	maxExitTimeout := time.Hour * 5

	if err = cmd.Start(); err != nil {
		return err
	}

	done := make(chan error)
	go func() {
		done <- cmd.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		d := time.Since(startTime)
		if err = cmd.Process.Kill(); err != nil {
			return err
		}
		select {
		case <-done:
		case <-time.After(maxExitTimeout):
			logrus.Warnf("Wait for command to exit timeout: %s", maxExitTimeout)
			return errors.Errorf("wait process to exit timeout after %s", maxExitTimeout)
		}
		if b, ok := cmd.Stdout.(*bytes.Buffer); ok {
			logrus.Warnf("Process was killed after %v: %s %s\nstdout: %v\nstderr: %v",
				d.Round(100*time.Millisecond), cmd.Path, cmd.Args, b, cmd.Stderr)
		} else {
			// Reduce time accuracy, avoid frequent log changes that affect logger rate limit
			logrus.Warnf("Process was killed after %v: %s %s\nstdout: %v\nstderr: %v",
				d.Round(100*time.Millisecond), cmd.Path, cmd.Args, cmd.Stdout, cmd.Stderr)
		}
		return ctx.Err()
	case err = <-done:
		return err
	}
}

// RunCommandFunc executes a command.
type RunCommandFunc func(ctx context.Context, command string, args ...string) (*bytes.Buffer, RunError)

var _ RunCommandFunc = realRun

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
