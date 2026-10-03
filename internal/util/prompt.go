package util

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/term"
)

// ErrInterrupted is returned by ReadKey when Ctrl-C or Ctrl-D is pressed. Raw
// mode turns these into ordinary bytes instead of signals, so they have to be
// recognised here to keep aborting the prompt.
var ErrInterrupted = errors.New("interrupted")

// ReadKey reads a single keypress from stdin without waiting for Enter. Enter
// itself is returned as '\n'.
//
// When stdin is not a terminal it reads a whole line and returns its first
// character, so piped answers such as `yes | mate apply` keep working. Reading
// byte by byte means nothing is buffered past the line, leaving the rest of the
// input for the next prompt.
func ReadKey() (byte, error) {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		var first byte
		b := make([]byte, 1)
		for i := 0; ; i++ {
			if _, err := os.Stdin.Read(b); err != nil {
				if i > 0 {
					return first, nil
				}
				return 0, err
			}
			if i == 0 {
				first = b[0]
			}
			if b[0] == '\n' {
				return first, nil
			}
		}
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return 0, err
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	b := make([]byte, 1)
	if _, err := os.Stdin.Read(b); err != nil {
		return 0, err
	}

	switch b[0] {
	case 3, 4: // Ctrl-C, Ctrl-D
		return 0, ErrInterrupted
	case '\r':
		return '\n', nil
	}
	return b[0], nil
}

// ReadLine reads one line from stdin and returns it without the newline. Like
// ReadKey it reads byte by byte, because a buffered reader would swallow the
// answers piped in for the prompts that follow.
func ReadLine() (string, error) {
	var line []byte
	b := make([]byte, 1)
	for {
		if _, err := os.Stdin.Read(b); err != nil {
			if len(line) > 0 {
				return string(line), nil
			}
			return "", err
		}
		if b[0] == '\n' {
			return string(line), nil
		}
		line = append(line, b[0])
	}
}

// Confirm prints prompt and waits for a single y or n keypress. Enter picks
// defaultYes; any other key counts as no.
func Confirm(prompt string, defaultYes bool) (bool, error) {
	fmt.Print(prompt)
	key, err := ReadKey()
	if err != nil {
		fmt.Println()
		return false, err
	}

	switch key {
	case 'y', 'Y':
		fmt.Println("y")
		return true, nil
	case '\n':
		fmt.Println()
		return defaultYes, nil
	default:
		fmt.Println("n")
		return false, nil
	}
}
