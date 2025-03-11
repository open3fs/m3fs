package external_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

// CommandCheckFunc is the type of check function can be add to mocked commands
type CommandCheckFunc func(mc *MockedCommands, command string, args ...string) (
	*bytes.Buffer, error)

// MockedCommandResult mocks command result
type MockedCommandResult struct {
	value     string
	err       error
	checkFunc CommandCheckFunc
	Called    bool
	Count     int
	times     int
}

// Value returns the value saved in MockedCommandResult
func (mcr *MockedCommandResult) Value() string {
	return mcr.value
}

// Error returns the error saved in MockedCommandResult
func (mcr *MockedCommandResult) Error() error {
	return mcr.err
}

// MockedCommandsResults defines mocked results
type MockedCommandsResults []*MockedCommandResult

// Count returns called times of all mocked results
func (rs MockedCommandsResults) Count() int {
	var count int
	for _, r := range rs {
		count += r.Count
	}
	return count
}

// Called checks if command is called
func (rs MockedCommandsResults) Called() bool {
	for _, r := range rs {
		if r.Called {
			return true
		}
	}
	return false
}

// MockedCommands mocks commands
type MockedCommands struct {
	t    *testing.T
	cmds map[string]MockedCommandsResults
}

// Add add mocked command prefix
func (mc *MockedCommands) Add(cmdPrefix string, returnValue string, returnError error,
	checkFunc CommandCheckFunc, times ...int) {

	mockedTimes := -1 // forever
	if len(times) > 0 {
		mockedTimes = times[0]
	}
	results, ok := mc.cmds[cmdPrefix]
	if !ok {
		results = MockedCommandsResults{}
	}
	if len(results) > 0 && results[0].times > 0 {
		results = append(results,
			&MockedCommandResult{
				value:     returnValue,
				err:       returnError,
				checkFunc: checkFunc,
				times:     mockedTimes,
			},
		)
	} else {
		results = MockedCommandsResults{
			&MockedCommandResult{
				value:     returnValue,
				err:       returnError,
				checkFunc: checkFunc,
				times:     mockedTimes,
			},
		}
	}
	mc.cmds[cmdPrefix] = results
}

// Mock mocks command for times
func (mc *MockedCommands) Mock(
	cmdPrefix string, returnValue string, returnError error, times ...int) {

	mc.Add(cmdPrefix, returnValue, returnError, nil, times...)
}

// MockOnce mocks command once
func (mc *MockedCommands) MockOnce(cmdPrefix string, returnValue string, returnError error) {
	mc.Mock(cmdPrefix, returnValue, returnError, 1)
}

// CalledCount returns called count for specific cmd prefix
func (mc *MockedCommands) CalledCount(cmdPrefix string) int {
	if _, ok := mc.cmds[cmdPrefix]; !ok {
		return 0
	}
	return mc.cmds[cmdPrefix].Count()
}

// Get returns mocked results for cmd prefix
func (mc *MockedCommands) Get(cmdPrefix string) MockedCommandsResults {
	return mc.cmds[cmdPrefix]
}

// Reset reset all mocked commands' result
func (mc *MockedCommands) Reset() {
	for k := range mc.cmds {
		for _, r := range mc.cmds[k] {
			r.Called = false
			r.Count = 0
		}
	}
}

// Run command with mocked command
func (mc *MockedCommands) Run(ctx context.Context, command string, args ...string) (
	*bytes.Buffer, error) {

	cmdLine := strings.Join(append([]string{command}, args...), " ")
	mc.t.Logf("Processing: %s", cmdLine)
	for cmdPrefix, results := range mc.cmds {
		if !strings.Contains(cmdLine, cmdPrefix) {
			continue
		}
		var result *MockedCommandResult
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
			return result.checkFunc(mc, command, args...)
		}
		return bytes.NewBufferString(result.value), result.err
	}

	return nil, fmt.Errorf("Unknown cmd: %s", cmdLine)
}

// NewMockedCommands creates new mocked command
func NewMockedCommands(t *testing.T) *MockedCommands {
	mc := &MockedCommands{
		t:    t,
		cmds: make(map[string]MockedCommandsResults),
	}
	return mc
}
