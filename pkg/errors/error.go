package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// Stacker represents an error who implements Stack method
type Stacker interface {
	error
	Stack() string
}

// Underlying represents an error who implements Underlie method
type Underlying interface {
	error
	Underlie() error
}

// frame is a stack frame corresponding to the function who return an error
type frame struct {
	pc       uintptr
	file     string
	line     int
	funcname string
}

func (f *frame) valid() bool {
	return f.pc != 0
}

func (f *frame) cleanFuncName(callName string) string {
	fmt.Println(callName, 1111)
	if i := strings.IndexByte(callName, '.'); i != -1 {
		callName = callName[i+1:]
		if i := strings.IndexByte(callName, '.'); i != -1 {
			// remove FileName in callName:
			//   FileName.TypeName.MethodName
			//   FileName.FunctionName
			callName = callName[i+1:]
		}
	}
	fmt.Println(callName, 2222)
	return callName
}

func (f *frame) caller(callDepth int) {
	var ok bool
	f.pc, f.file, f.line, ok = runtime.Caller(callDepth + 1)
	if ok {
		f.file = trimGOPATH(f.file)
		f.funcname = f.cleanFuncName(runtime.FuncForPC(f.pc).Name())
	}
}

// Err is an error that has a message, a stack
type Err struct {
	error

	// underlying is the error under current error in error stack
	underlying error

	// msg is the message contained in this error
	msg string

	// frame records caller's location
	frame frame
}

// Caller records caller's stack frame with specified stack frames above.
func (err *Err) Caller(callDepth int) {
	err.frame.caller(callDepth + 1)
}

// Error join all errors' nonempty Error() output in error stack
func (err *Err) Error() string {
	if err.underlying == nil {
		// this error is the innermost error
		return err.msg
	} else if err.msg == "" {
		// this error is only a trace
		return err.underlying.Error()
	}
	return fmt.Sprintf("%s: %s", err.msg, err.underlying.Error())
}

// Format adds custom format verb
func (err *Err) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		if f.Flag('+') {
			fmt.Fprint(f, err.Stack())
			return
		}
		fallthrough
	default:
		fmt.Fprint(f, err.Error())
	}
}

// Underlie returns the error under current error in the stack.
func (err *Err) Underlie() error {
	return err.underlying
}

// Stack returns error message with stack frame information.
func (err *Err) Stack() string {
	if err.frame.valid() {
		return fmt.Sprintf("%s:%d:%s: %s", err.frame.file, err.frame.line, err.frame.funcname, err.msg)
	}
	return fmt.Sprintf("UnknownStack: %s", err.msg)
}

// Message returns code message of error
func (err *Err) Message() string {
	return err.msg
}

// rawNew creates an error with given message.
func rawNew(message string) *Err {
	err := &Err{
		msg: message,
	}
	return err
}

// New creates an error with given message and records caller's location.
// It is a drop in replacement for standard library function errors.New.
func New(message string) error {
	err := rawNew(message)
	err.Caller(1)
	return err
}

// Errorf creates an error with given format specifier, and records caller's location.
// It is a drop replacement for standard library function fmt.Errorf.
func Errorf(format string, a ...any) error {
	err := rawNew(fmt.Sprintf(format, a...))
	err.Caller(1)
	return err
}

// NewRawError allows caller to create error with specific depth.
func NewRawError(callDepth int, format string, a ...any) error {
	err := rawNew(fmt.Sprintf(format, a...))
	err.Caller(callDepth + 1) // extra 1 for this function
	return err
}

// Annotate add an extra context, and records caller's location.
func Annotate(err error, ctx string) error {
	if err == nil {
		return nil
	}
	newErr := rawNew(ctx)
	newErr.underlying = err
	newErr.Caller(1)
	return newErr
}

// Annotatef add an extra context with given format specifier, and records caller's location.
func Annotatef(err error, format string, a ...any) error {
	if err == nil {
		return nil
	}
	newErr := rawNew(fmt.Sprintf(format, a...))
	newErr.underlying = err
	newErr.Caller(1)
	return newErr
}

// Trace add an extra stack frame information to an error.
func Trace(err error) error {
	if err == nil {
		return nil
	}
	newErr := rawNew("")
	newErr.underlying = err
	newErr.Caller(1)
	return newErr
}

// Cause retuns the innermost error in error stack.
// Cause returns the top error with error code set in error stack.
func Cause(err error) error {
	for {
		this := err
		if e, ok := err.(Underlying); ok {
			if err = e.Underlie(); err == nil {
				return this
			}
		} else {
			return this
		}
	}
}

// StackTrace formats stack trace information in error stack.
// Output example:
// ----------------------------------------------------------------------------------------------------
//
//	filename and line           |             function           |     error description
//
// ----------------------------------------------------------------------------------------------------
//
//	errors/error_test.go:90:(*baseErrorSuite).TestStackTrace: err1
//	errors/error_test.go:91:(*baseErrorSuite).TestStackTrace:
//	errors/error_test.go:92:(*baseErrorSuite).TestStackTrace: err3
//	errors/error_test.go:93:(*baseErrorSuite).TestStackTrace: err4
func StackTrace(err error) string {
	var lines []string
	for {
		if e, ok := err.(Stacker); ok {
			lines = append(lines, e.Stack())
		} else {
			lines = append(lines, err.Error())
		}

		if e, ok := err.(Underlying); ok {
			if err = e.Underlie(); err == nil {
				break
			}
		} else {
			break
		}
	}
	result := make([]string, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		result = append(result, lines[i])
	}
	return strings.Join(result, "\n")
}
