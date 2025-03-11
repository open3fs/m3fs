package external_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/open3fs/m3fs/pkg/external"
)

var suiteRun = suite.Run

type Suite struct {
	suite.Suite

	mc *MockedCommands
	em *external.Manager
}

func (s *Suite) SetupSuite() {
	s.T().Parallel()
}

func (s *Suite) SetupTest() {
	s.mc = NewMockedCommands(s.T())
	s.em = external.NewManager(s.mc)
}

// Ctx returns a context used in test.
func (s *Suite) Ctx() context.Context {
	return context.TODO()
}

// R returns a require context.
func (s *Suite) R() *require.Assertions {
	return s.Require()
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
