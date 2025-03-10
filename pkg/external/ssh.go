package external

// SSHInterface provides interface about ssh.
type SSHInterface interface {
	ExecCommand(cmd string) (string, error)
}

type sshExternal struct {
	externalBase
}

func (se *sshExternal) init(em *Manager) {
	se.externalBase.init(em)
	em.SSH = se
}

func (se *sshExternal) ExecCommand(cmd string) (string, error) {
	// TODO: implement ssh command execution
	return "", nil
}

func init() {
	registerNewExternalFunc(func() externalInterface {
		return new(sshExternal)
	})
}
