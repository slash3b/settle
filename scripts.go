package main

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
)

// IsScriptInstalled checks if a binary is available in PATH.
func IsScriptInstalled(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// RunScript executes a shell command string via sh -c.
// In verbose mode output streams live; otherwise output is suppressed and only shown on failure.
func RunScript(script string, verbose bool, w io.Writer) error {
	if verbose {
		_, _ = fmt.Fprintf(w, "Running: %s\n", script)
	}

	cmd := exec.Command("sh", "-c", script) //nolint:gosec // script is user-supplied config

	if verbose {
		cmd.Stdout = w
		cmd.Stderr = w
	} else {
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf

		if err := cmd.Run(); err != nil {
			_, _ = w.Write(buf.Bytes())
			return fmt.Errorf("script failed: %w", err)
		}

		return nil
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("script failed: %w", err)
	}

	return nil
}
