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

package external_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ExecCheckFunc is the type of check function for Exec.
type ExecCheckFunc func(mr *MockedRunner, command string, args ...string) (string, error)

// MockedExecResult mocks result of the Exec func
type MockedExecResult struct {
	value     string
	err       error
	checkFunc ExecCheckFunc
	Called    bool
	Count     int
	times     int
}

// Value returns the value saved in MockedExecResult
func (r *MockedExecResult) Value() string {
	return r.value
}

// Error returns the error saved in MockedExecResult
func (r *MockedExecResult) Error() error {
	return r.err
}

// MockedExecResults defines mocked exec results
type MockedExecResults []*MockedExecResult

// Count returns called times of all mocked results
func (rs MockedExecResults) Count() int {
	var count int
	for _, r := range rs {
		count += r.Count
	}
	return count
}

// Called checks if Exec func is called
func (rs MockedExecResults) Called() bool {
	for _, r := range rs {
		if r.Called {
			return true
		}
	}
	return false
}

// ScpCheckFunc is the type of check function for Scp.
type ScpCheckFunc func(mc *MockedRunner, local, remote string) error

// MockedScpResult mocks result of the Scp func
type MockedScpResult struct {
	err       error
	checkFunc ScpCheckFunc
	Called    bool
	Count     int
	times     int
}

// Error returns the error saved in MockedScpResult
func (r *MockedScpResult) Error() error {
	return r.err
}

// MockedScpResults defines mocked scp results
type MockedScpResults []*MockedScpResult

// Count returns called times of all mocked results
func (rs MockedScpResults) Count() int {
	var count int
	for _, r := range rs {
		count += r.Count
	}
	return count
}

// Called checks if Scp func is called
func (rs MockedScpResults) Called() bool {
	for _, r := range rs {
		if r.Called {
			return true
		}
	}
	return false
}

// MockedRunner mocks RunInterface
type MockedRunner struct {
	t           *testing.T
	execResults map[string]MockedExecResults
	scpResults  map[string]MockedScpResults
}

// Reset reset all mocked results
func (mr *MockedRunner) Reset() {
	for k := range mr.execResults {
		for _, r := range mr.execResults[k] {
			r.Called = false
			r.Count = 0
		}
	}
	for k := range mr.scpResults {
		for _, r := range mr.scpResults[k] {
			r.Called = false
			r.Count = 0
		}
	}
}

// Add add mocked command prefix
func (mr *MockedRunner) AddExec(cmdPrefix string, returnValue string, returnError error,
	checkFunc ExecCheckFunc, times ...int) {

	mockedTimes := -1 // forever
	if len(times) > 0 {
		mockedTimes = times[0]
	}
	results, ok := mr.execResults[cmdPrefix]
	if !ok {
		results = MockedExecResults{}
	}
	if len(results) > 0 && results[0].times > 0 {
		results = append(results,
			&MockedExecResult{
				value:     returnValue,
				err:       returnError,
				checkFunc: checkFunc,
				times:     mockedTimes,
			},
		)
	} else {
		results = MockedExecResults{
			&MockedExecResult{
				value:     returnValue,
				err:       returnError,
				checkFunc: checkFunc,
				times:     mockedTimes,
			},
		}
	}
	mr.execResults[cmdPrefix] = results
}

// Mock mocks command for times
func (mr *MockedRunner) MockExec(
	cmdPrefix string, returnValue string, returnError error, times ...int) {

	mr.AddExec(cmdPrefix, returnValue, returnError, nil, times...)
}

// MockOnce mocks command once
func (mr *MockedRunner) MockExecOnce(cmdPrefix string, returnValue string, returnError error) {
	mr.MockExec(cmdPrefix, returnValue, returnError, 1)
}

// CalledExecCount returns called Exec count for specific cmd prefix
func (mr *MockedRunner) CalledExecCount(cmdPrefix string) int {
	if _, ok := mr.execResults[cmdPrefix]; !ok {
		return 0
	}
	return mr.execResults[cmdPrefix].Count()
}

// NonSudoExec executes a command.
func (mr *MockedRunner) NonSudoExec(ctx context.Context, command string, args ...string) (string, error) {
	cmdLine := strings.Join(append([]string{command}, args...), " ")
	mr.t.Logf("Processing: %s", cmdLine)
	for cmdPrefix, results := range mr.execResults {
		if !strings.Contains(cmdLine, cmdPrefix) {
			continue
		}
		var result *MockedExecResult
		var found bool
		for _, result = range results {
			if result.times != 0 {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		result.Called = true
		result.Count++
		if result.times > 0 {
			result.times--
		}
		if result.checkFunc != nil {
			return result.checkFunc(mr, command, args...)
		}
		return result.value, result.err
	}

	return "", fmt.Errorf("Unknown cmd: %s", cmdLine)
}

func (mr *MockedRunner) Exec(ctx context.Context, command string, args ...string) (string, error) {
	cmdLine := strings.Join(append([]string{command}, args...), " ")
	mr.t.Logf("Processing: %s", cmdLine)
	for cmdPrefix, results := range mr.execResults {
		if !strings.Contains(cmdLine, cmdPrefix) {
			continue
		}
		var result *MockedExecResult
		var found bool
		for _, result = range results {
			if result.times != 0 {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		result.Called = true
		result.Count++
		if result.times > 0 {
			result.times--
		}
		if result.checkFunc != nil {
			return result.checkFunc(mr, command, args...)
		}
		return result.value, result.err
	}

	return "", fmt.Errorf("Unknown cmd: %s", cmdLine)
}

// Add add mocked command prefix
func (mr *MockedRunner) AddScp(local, remote string, returnError error,
	checkFunc ScpCheckFunc, times ...int) {

	resultKey := local + ":" + remote
	mockedTimes := -1 // forever
	if len(times) > 0 {
		mockedTimes = times[0]
	}
	results, ok := mr.scpResults[resultKey]
	if !ok {
		results = MockedScpResults{}
	}
	if len(results) > 0 && results[0].times > 0 {
		results = append(results,
			&MockedScpResult{
				err:       returnError,
				checkFunc: checkFunc,
				times:     mockedTimes,
			},
		)
	} else {
		results = MockedScpResults{
			&MockedScpResult{
				err:       returnError,
				checkFunc: checkFunc,
				times:     mockedTimes,
			},
		}
	}
	mr.scpResults[resultKey] = results
}

// Mock mocks command for times
func (mr *MockedRunner) MockScp(local, remote string, returnError error, times ...int) {
	mr.AddScp(local, remote, returnError, nil, times...)
}

// MockOnce mocks command once
func (mr *MockedRunner) MockScpOnce(local, remote string, returnError error) {
	mr.MockScp(local, remote, returnError, 1)
}

// CalledScpCount returns called Scp count for specific cmd prefix
func (mr *MockedRunner) CalledScpCount(local, remote string) int {
	resultKey := local + ":" + remote
	if _, ok := mr.scpResults[resultKey]; !ok {
		return 0
	}
	return mr.scpResults[resultKey].Count()
}

// Scp copy local file or dir to remote host.
func (mr *MockedRunner) Scp(ctx context.Context, local, remote string) error {
	mr.t.Logf("Processing: scp %s to %s", local, remote)
	resultKey := local + ":" + remote
	if results, ok := mr.scpResults[resultKey]; ok {
		var result *MockedScpResult
		var found bool
		for _, result = range results {
			if result.times != 0 {
				found = true
				break
			}
		}
		if found {
			result.Called = true
			result.Count++
			if result.times > 0 {
				result.times--
			}
			if result.checkFunc != nil {
				return result.checkFunc(mr, local, remote)
			}
			return result.err
		}
	}
	return fmt.Errorf("Unexpected scp from %s to %s", local, remote)
}

// NewMockedRunner creates new mocked runner.
func NewMockedRunner(t *testing.T) *MockedRunner {
	mr := &MockedRunner{
		t:           t,
		execResults: make(map[string]MockedExecResults),
		scpResults:  make(map[string]MockedScpResults),
	}
	return mr
}
