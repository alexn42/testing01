// Package exectool runs the external CLI tools (ffmpeg, poppler-utils,
// tesseract) mediasearch shells out to, turning a missing binary or a
// nonzero exit into an apperr with a precise, actionable message.
package exectool

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"

	"mediasearch/internal/apperr"
)

// Run executes name with args and returns its stdout. A missing binary and
// a nonzero exit are both reported as an *apperr.Error.
func Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return nil, apperr.New(
				"required tool '%s' was not found on PATH (see README for runtime dependencies)", name,
			)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, apperr.New("%s failed: %s", name, msg)
	}
	return stdout.Bytes(), nil
}
