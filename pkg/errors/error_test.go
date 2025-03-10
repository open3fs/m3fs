package errors

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

const (
	thisFile = "pkg/errors/error_test.go"
)

func TestBaseErrorSuite(t *testing.T) {
	suite.Run(t, new(baseErrorSuite))
}

type baseErrorSuite struct {
	Suite
}

func (s *baseErrorSuite) TestBasicError() {
	r := s.R()

	err := New("basic")
	r.Equal("basic", err.Error())
	r.Nil(err.(Underlying).Underlie())
	r.Nil(err.(Underlying).Underlie())
	r.Equal(fmt.Sprintf("%s:%d:(*baseErrorSuite).TestBasicError: basic", thisFile, err.(*Err).frame.line),
		fmt.Sprintf("%+v", err))

	err = Errorf("basic %d", 10)
	r.Equal("basic 10", err.Error())
	r.Nil(err.(Underlying).Underlie())
	r.Equal(fmt.Sprintf("%s:%d:(*baseErrorSuite).TestBasicError: basic 10", thisFile, err.(*Err).frame.line),
		fmt.Sprintf("%+v", err))
}

func (s *baseErrorSuite) TestAnnotateNormalError() {
	r := s.R()

	err1 := fmt.Errorf("err1")
	err2 := Trace(err1)
	err3 := Annotate(err2, "err3")

	// Cause
	r.Equal(err1, Cause(err1))
	r.Equal(err1, Cause(err2))
	r.Equal(err1, Cause(err3))
}

func (s *baseErrorSuite) TestStackTrace() {
	r := s.R()

	err1 := New("err1")
	err2 := Trace(err1)
	err3 := Annotate(err2, "err3")
	err4 := Annotatef(err3, "err%d", 4)

	// Cause
	r.Equal(err1, Cause(err1))
	r.Equal(err1, Cause(err2))
	r.Equal(err1, Cause(err3))
	r.Equal(err1, Cause(err4))

	// StackTrace
	err1Line := err1.(*Err).frame.line
	r.NotEmpty(err1Line)
	err2Line := err1Line + 1
	err3Line := err1Line + 2
	err4Line := err1Line + 3
	result := StackTrace(err4)
	lines := strings.Split(result, "\n")
	r.Len(lines, 4)
	r.Equal(fmt.Sprintf("%s:%d:(*baseErrorSuite).TestStackTrace: err1", thisFile, err1Line), lines[0])
	r.Equal(fmt.Sprintf("%s:%d:(*baseErrorSuite).TestStackTrace: ", thisFile, err2Line), lines[1])
	r.Equal(fmt.Sprintf("%s:%d:(*baseErrorSuite).TestStackTrace: err3", thisFile, err3Line), lines[2])
	r.Equal(fmt.Sprintf("%s:%d:(*baseErrorSuite).TestStackTrace: err4", thisFile, err4Line), lines[3])
}

func (s *baseErrorSuite) TestStackTraceWithNormalError() {
	r := s.R()

	err1 := fmt.Errorf("err1")
	err2 := Trace(err1)
	err3 := Annotate(err2, "err3")
	err4 := Annotatef(err3, "err%d", 4)

	// Cause
	r.Equal(err1, Cause(err1))
	r.Equal(err1, Cause(err2))
	r.Equal(err1, Cause(err3))
	r.Equal(err1, Cause(err4))

	// StackTrace
	err2Line := err2.(*Err).frame.line
	r.NotEmpty(err2Line)
	err3Line := err2Line + 1
	err4Line := err2Line + 2
	result := StackTrace(err4)
	lines := strings.Split(result, "\n")
	r.Len(lines, 4)
	r.Equal("err1", lines[0])
	r.Equal(fmt.Sprintf("%s:%d:(*baseErrorSuite).TestStackTraceWithNormalError: ", thisFile, err2Line), lines[1])
	r.Equal(fmt.Sprintf("%s:%d:(*baseErrorSuite).TestStackTraceWithNormalError: err3", thisFile, err3Line), lines[2])
	r.Equal(
		fmt.Sprintf("%s:%d:(*baseErrorSuite).TestStackTraceWithNormalError: err4", thisFile, err4Line),
		lines[3])
}

// TestError error
type TestError struct {
	*Err
	Field string
}

// NewTestError error
func NewTestError(message string) error {
	err := &TestError{}
	err.Field = "field"
	err.Err = rawNew("[TestError] " + message + " " + err.Field)
	err.Caller(1)
	return err
}

func TestCustomErrorSuite(t *testing.T) {
	suite.Run(t, new(customErrorSuite))
}

type customErrorSuite struct {
	Suite
}

func (s *customErrorSuite) Test() {
	r := s.R()

	err := NewTestError("test message")
	r.Equal("[TestError] test message field", err.Error())
	r.Nil(err.(Underlying).Underlie())
}

func (s *customErrorSuite) TestStackTrace() {
	r := s.R()

	err1 := NewTestError("test message")
	err2 := Trace(err1)
	err3 := Annotate(err2, "err3")
	err4 := Annotatef(err3, "err%d", 4)

	// Cause
	r.Equal(err1, Cause(err1))
	r.Equal(err1, Cause(err2))
	r.Equal(err1, Cause(err3))
	r.Equal(err1, Cause(err4))

	// StackTrace
	err1Line := err1.(*TestError).frame.line
	r.NotEmpty(err1Line)
	err2Line := err1Line + 1
	err3Line := err1Line + 2
	err4Line := err1Line + 3
	result := StackTrace(err4)
	lines := strings.Split(result, "\n")
	r.Len(lines, 4)
	r.Equal(fmt.Sprintf("%s:%d:(*customErrorSuite).TestStackTrace: [TestError] test message field", thisFile, err1Line), lines[0])
	r.Equal(fmt.Sprintf("%s:%d:(*customErrorSuite).TestStackTrace: ", thisFile, err2Line), lines[1])
	r.Equal(fmt.Sprintf("%s:%d:(*customErrorSuite).TestStackTrace: err3", thisFile, err3Line), lines[2])
	r.Equal(fmt.Sprintf("%s:%d:(*customErrorSuite).TestStackTrace: err4", thisFile, err4Line), lines[3])
}
