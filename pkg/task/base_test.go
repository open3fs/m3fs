package task

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/tests/base"
)

type baseSuite struct {
	base.Suite
}

var suiteRun = suite.Run

type mockTask struct {
	mock.Mock
	Interface
}

func (m *mockTask) Run(context.Context) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockTask) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockTask) Init(r *Runtime) {
	m.Called(r)
}
