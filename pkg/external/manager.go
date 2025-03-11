package external

import (
	"bytes"
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/open3fs/m3fs/pkg/config"
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
	OS     OSInterface
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

var remoteManagerCache sync.Map

// NewRemoteRunnerManager create a new remote runner manager
func NewRemoteRunnerManager(node *config.Node) (*Manager, error) {
	mgr, ok := remoteManagerCache.Load(node)
	if ok {
		return mgr.(*Manager), nil
	}
	runner, err := NewRemoteRunner(&RemoteRunnerCfg{
		Username:   node.Username,
		Password:   node.Password,
		TargetHost: node.Host,
		TargetPort: node.Port,
		// TODO: add timeout config
	})
	if err != nil {
		return nil, errors.Annotatef(err, "create remote runner for node [%s]", node.Name)
	}

	return NewManager(runner), nil
}
