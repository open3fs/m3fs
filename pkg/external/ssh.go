package external

import (
	"context"

	"github.com/open3fs/m3fs/pkg/errors"
)

// SSHInterface provides interface about ssh.
type SSHInterface interface {
	ExecCommand(context.Context, string, string, ...string) (string, error)
}

type sshExternal struct {
	externalBase
}

func (se *sshExternal) init(em *Manager) {
	se.externalBase.init(em)
	em.SSH = se
}

func (se *sshExternal) ExecCommand(ctx context.Context, host, cmd string, args ...string) (string, error) {
	cmdArgs := append([]string{host, cmd}, args...)
	out, err := se.run(ctx, "ssh", cmdArgs...)
	if err != nil {
		return "", errors.Trace(err)
	}
	return out.String(), nil
}

func init() {
	registerNewExternalFunc(func() externalInterface {
		return new(sshExternal)
	})
}
