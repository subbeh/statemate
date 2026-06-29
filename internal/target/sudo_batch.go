package target

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

type SudoBatch struct {
	script bytes.Buffer
	files  map[string][]byte
}

func NewSudoBatch() *SudoBatch {
	return &SudoBatch{
		files: make(map[string][]byte),
	}
}

func (b *SudoBatch) AddMkdir(path string, mode os.FileMode) {
	b.script.WriteString(fmt.Sprintf("mkdir -p %q && chmod %o %q\n", path, mode, path))
}

func (b *SudoBatch) AddWriteFile(path string, content []byte, mode os.FileMode) {
	tmpName := fmt.Sprintf("/tmp/statemate-%d-%d", os.Getpid(), len(b.files))
	b.files[tmpName] = content
	b.script.WriteString(fmt.Sprintf("cp %q %q && chmod %o %q && rm %q\n", tmpName, path, mode, path, tmpName))
}

func (b *SudoBatch) AddChown(path, owner, group string) {
	if group != "" {
		b.script.WriteString(fmt.Sprintf("chown %s:%s %q\n", owner, group, path))
	} else if owner != "" {
		b.script.WriteString(fmt.Sprintf("chown %s %q\n", owner, path))
	}
}

func (b *SudoBatch) AddChmod(path string, mode os.FileMode) {
	b.script.WriteString(fmt.Sprintf("chmod %o %q\n", mode, path))
}

func (b *SudoBatch) IsEmpty() bool {
	return b.script.Len() == 0
}

func (b *SudoBatch) Execute() error {
	if b.IsEmpty() {
		return nil
	}

	for tmpPath, content := range b.files {
		if err := os.WriteFile(tmpPath, content, 0600); err != nil {
			return fmt.Errorf("writing temp file %s: %w", tmpPath, err)
		}
		defer os.Remove(tmpPath)
	}

	cmd := exec.Command("sudo", "sh", "-c", b.script.String())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (b *SudoBatch) String() string {
	return b.script.String()
}
