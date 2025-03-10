package errors

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type Suite struct {
	suite.Suite
}

// R returns a require context.
func (s *Suite) R() *require.Assertions {
	return s.Require()
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
