package external

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// Command define command
type Command struct {
	runner RunInterface

	cmdName string
	args    []string
}

// Command gets the command
func (cmd *Command) Command() string {
	if len(cmd.args) > 0 {
		return cmd.cmdName + " " + strings.Join(cmd.args, " ")
	}
	return cmd.cmdName
}

// AppendArgs append new args to current args
func (cmd *Command) AppendArgs(args ...any) {
	for _, arg := range args {
		cmd.args = append(cmd.args, fmt.Sprintf("%v", arg))
	}
}

// Execute execute the command
func (cmd *Command) Execute(ctx context.Context) (out *bytes.Buffer, err error) {
	if cmd.cmdName == "" {
		return nil, fmt.Errorf("No command")
	}
	return cmd.runner.Run(ctx, cmd.cmdName, cmd.args...)
}

func (cmd *Command) String() string {
	return fmt.Sprintf("cmd: %s", cmd.Command())
}

// NewCommand inits a new command
func NewCommand(cmdName string, args ...any) *Command {
	cmd := &Command{
		cmdName: cmdName,
	}
	cmd.AppendArgs(args...)

	return cmd
}
