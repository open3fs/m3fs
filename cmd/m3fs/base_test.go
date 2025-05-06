package main

import (
	"github.com/stretchr/testify/suite"

	tbase "github.com/open3fs/m3fs/tests/base"
)

var suiteRun = suite.Run

type Suite struct {
	tbase.Suite
}

func (s *Suite) SetupTest() {
	s.Suite.SetupTest()
}
