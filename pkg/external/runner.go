package external

import (
	"bytes"
	"context"
)

// RunInterface is the interface for running command.
type RunInterface interface {
	Run(ctx context.Context, command string, args ...string) (*bytes.Buffer, error)
}

// RemoteRunner implements RunInterface by running command on a remote host.
type RemoteRunner struct {
}

// Run is used for running a command.
func (r *RemoteRunner) Run(ctx context.Context, command string, args ...string) (
	*bytes.Buffer, error) {

	return nil, nil
}
