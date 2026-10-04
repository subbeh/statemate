package util

import "os/exec"

// UserCommand runs a command line the user configured, such as $EDITOR or
// diff_tool, with args appended. The line goes through sh, as git does for
// $EDITOR, so "code --wait" and a quoted path with spaces both work; executed
// as a bare program name, they failed with "executable file not found".
func UserCommand(command string, args ...string) *exec.Cmd {
	return exec.Command("sh", append([]string{"-c", command + ` "$@"`, command}, args...)...)
}
