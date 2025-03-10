package errors

import (
	"runtime"
	"strings"
)

var (
	goPath    string
	goPathLen int
)

func init() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	size := len(file)
	suffixLen := len("pkg/errors/path.go") // also trim project directory here
	goPath = file[:size-suffixLen]
	goPathLen = len(goPath)
}

func trimGOPATH(path string) string {
	if strings.HasPrefix(path, goPath) {
		return path[goPathLen:]
	}
	return path
}
