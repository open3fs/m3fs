package common

import (
	"github.com/davecgh/go-spew/spew"
)

var (
	spewConfig = &spew.ConfigState{
		Indent:         "\t",
		MaxDepth:       3,
		DisableMethods: true,
	}
)

// PrettyDump prints Golang objects in a beautiful way.
func PrettyDump(a ...any) {
	spewConfig.Dump(a...)
}

// PrettySdump prints Golang objects in a beautiful way to string.
func PrettySdump(a ...any) string {
	return spewConfig.Sdump(a...)
}
