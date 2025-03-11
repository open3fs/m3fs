package external_test

import (
	"testing"
)

func TestOSExecSuite(t *testing.T) {
	suiteRun(t, new(osExecSuite))
}

type osExecSuite struct {
	Suite
}

func (s *osExecSuite) Test() {
	mockCmd := "mkdir -p /tmp/m3fs"
	s.mc.Mock(mockCmd, "", nil)
	_, err := s.em.OS.Exec(s.Ctx(), "mkdir", "-p", "/tmp/m3fs")
	s.NoError(err)
}
