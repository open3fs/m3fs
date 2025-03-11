package external

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/open3fs/m3fs/pkg/external"
)

// MockOS is a mock type for the external.OSInterface interface.
type MockOS struct {
	mock.Mock
	external.OSInterface
}

// Exec mock.
func (m *MockOS) Exec(ctx context.Context, cmd string, arg ...string) (string, error) {
	args := m.Called(cmd, arg)
	return args.String(0), args.Error(1)
}
