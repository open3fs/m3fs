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
	"context"

	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/task"
)

type cleanupLocalStep struct {
	task.BaseStep

	tmpDirKey string
}

func (s *cleanupLocalStep) Execute(ctx context.Context) error {
	tmpDir, ok := s.Runtime.LoadString(s.tmpDirKey)
	if !ok {
		return errors.Errorf("Failed to get value of %s", s.tmpDirKey)
	}
	if err := s.Runtime.LocalEm.FS.RemoveAll(ctx, tmpDir); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// NewCleanupLocalStepFunc is the cleanup local step factory func.
func NewCleanupLocalStepFunc(tmpDirKey string) func() task.Step {
	return func() task.Step {
		return &cleanupLocalStep{
			tmpDirKey: tmpDirKey,
		}
	}
}
