package external

import (
	"bytes"
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/open3fs/m3fs/pkg/errors"
)

type externalInterface interface {
	init(em *Manager)
}

type externalBase struct {
	em  *Manager
	log *log.Logger
}

func (eb *externalBase) init(em *Manager) {
	eb.em = em
	eb.log = log.StandardLogger()
}

func (eb *externalBase) runWithAny(ctx context.Context, cmdName string, args ...any) (*bytes.Buffer, error) {
	cmd := NewCommand(cmdName, args...)
	cmd.runner = eb.em.runner
	out, err := cmd.Execute(ctx)
	if err != nil {
		return out, errors.Annotatef(err, "run cmd [%s]", cmd.String())
	}
	return out, nil
}

func (eb *externalBase) run(ctx context.Context, cmdName string, args ...string) (
	*bytes.Buffer, error) {

	anyArgs := []any{}
	for _, arg := range args {
		anyArgs = append(anyArgs, arg)
	}
	return eb.runWithAny(ctx, cmdName, anyArgs...)
}

// create a new external
type newExternalFunc func() externalInterface

var (
	newExternals []newExternalFunc
	lock         sync.Mutex
)

func registerNewExternalFunc(f newExternalFunc) {
	lock.Lock()
	defer lock.Unlock()
	newExternals = append(newExternals, f)
}

// Manager provides a way to use all external interfaces
type Manager struct {
	runner RunInterface

	Net    NetInterface
	Docker DockerInterface
	Disk   DiskInterface
	SSH    SSHInterface
}

// NewManagerFunc type of new manager func.
type NewManagerFunc func() *Manager

// NewManager create a new external manager
func NewManager(runner RunInterface) (em *Manager) {
	em = &Manager{
		runner: runner,
	}
	for _, newExternal := range newExternals {
		newExternal().init(em)
	}
	return em
}
