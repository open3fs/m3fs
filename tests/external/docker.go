package external

import (
	"bytes"
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/open3fs/m3fs/pkg/external"
)

// MockDocker is an mock type for the DockerInterface
type MockDocker struct {
	mock.Mock
	external.DockerInterface
}

// Run mock.
func (m *MockDocker) Run(ctx context.Context, args *external.RunArgs) (*bytes.Buffer, error) {
	arg := m.Called(args)
	err1 := arg.Error(1)
	if err1 != nil {
		return nil, err1
	}

	return arg.Get(0).(*bytes.Buffer), nil
}
