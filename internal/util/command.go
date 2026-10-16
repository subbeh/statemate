package util

import (
	"log/slog"
	"os/exec"
	"time"
)

// Cmd is an exec.Cmd that writes what it ran, and how that ended, to the log
// (--log-file). Only the arguments are logged: output and environment can hold
// secrets, such as the session key 'bw unlock --raw' prints.
type Cmd struct {
	*exec.Cmd
}

// Command is exec.Command, logged. Every external program mate runs goes
// through it, so the log shows each one.
func Command(name string, args ...string) *Cmd {
	return &Cmd{exec.Command(name, args...)}
}

func (c *Cmd) Run() error {
	start := time.Now()
	return c.log(start, c.Cmd.Run())
}

func (c *Cmd) Output() ([]byte, error) {
	start := time.Now()
	out, err := c.Cmd.Output()
	return out, c.log(start, err)
}

func (c *Cmd) CombinedOutput() ([]byte, error) {
	start := time.Now()
	out, err := c.Cmd.CombinedOutput()
	return out, c.log(start, err)
}

func (c *Cmd) log(start time.Time, err error) error {
	exit := -1
	if c.ProcessState != nil {
		exit = c.ProcessState.ExitCode()
	}
	attrs := []any{"args", c.Args, "dir", c.Dir, "exit", exit, "took", time.Since(start).Round(time.Millisecond)}
	if err != nil {
		attrs = append(attrs, "err", err)
	}
	slog.Info("exec", attrs...)
	return err
}

// UserCommand runs a command line the user configured, such as $EDITOR or
// diff_tool, with args appended. The line goes through sh, as git does for
// $EDITOR, so "code --wait" and a quoted path with spaces both work; executed
// as a bare program name, they failed with "executable file not found".
func UserCommand(command string, args ...string) *Cmd {
	return Command("sh", append([]string{"-c", command + ` "$@"`, command}, args...)...)
}
