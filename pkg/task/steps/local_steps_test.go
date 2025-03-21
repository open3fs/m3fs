// Copyright 2025 Open3FS Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package steps

import (
	"testing"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/task"
	ttask "github.com/open3fs/m3fs/tests/task"
)

func TestCleanupLocalStepSuite(t *testing.T) {
	suiteRun(t, &cleanupLocalStepSuite{})
}

type cleanupLocalStepSuite struct {
	ttask.StepSuite

	step *cleanupLocalStep
}

func (s *cleanupLocalStepSuite) SetupTest() {
	s.StepSuite.SetupTest()

	s.step = NewCleanupLocalStepFunc(task.RuntimeClickhouseTmpDirKey)().(*cleanupLocalStep)
	s.SetupRuntime()
	s.step.Init(s.Runtime, s.MockEm, config.Node{}, s.Logger)
	s.Runtime.Store(task.RuntimeClickhouseTmpDirKey, "/tmp/3fs/clickhouse-xxx")
}

func (s *cleanupLocalStepSuite) Test() {
	s.MockLocalFS.On("RemoveAll", "/tmp/3fs/clickhouse-xxx").Return(nil)

	s.NoError(s.step.Execute(s.Ctx()))
}
