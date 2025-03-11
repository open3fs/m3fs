package external

import (
	"context"

	"github.com/open3fs/m3fs/pkg/errors"
)

// OSInterface provides interface about os.
type OSInterface interface {
	Exec(context.Context, string, ...string) (string, error)
}

type osExternal struct {
	externalBase
}

func (oe *osExternal) init(em *Manager) {
	oe.externalBase.init(em)
	em.OS = oe
}

func (oe *osExternal) Exec(ctx context.Context, cmd string, args ...string) (string, error) {
	out, err := oe.run(ctx, cmd, args...)
	if err != nil {
		return "", errors.Trace(err)
	}
	return out.String(), nil
}

func init() {
	registerNewExternalFunc(func() externalInterface {
		return new(osExternal)
	})
}
