package helpers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mitchellh/go-linereader"

	"github.com/jorgerojas26/lazysql/helpers/logger"
)

// [doneFn] is invoked when the [command] is completed with its stdout.
func RunCommand(ctx context.Context, command string, doneFn func(output string)) error {
	var cmd *exec.Cmd

	// Execute the command using a shell to handle pipes and other shell features.
	// Use /bin/sh on Linux/macOS and cmd.exe on Windows.
	if isWindows() {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", command) // #nosec G204
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command) // #nosec G204
	}

	// Create a pipe to read the output from.
	pr, pw := io.Pipe()
	startedCh := make(chan struct{})
	finishedCh := make(chan struct{})
	go logOutput(pr, startedCh, finishedCh)

	// Connect the pipe to stdout and stderr.
	cmd.Stderr = pw
	// Hook in to stdout. Write to the pipe for logging purposes and
	// to the buffer for [doneFn]'s argument.
	stdoutBytes := bytes.Buffer{}
	cmd.Stdout = io.MultiWriter(pw, &stdoutBytes)

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			logger.Error("Command stopped", map[string]any{"error": err.Error()})
		}

		_ = pw.Close()
		<-finishedCh

		// Trim the carriage return added by the use of a shell.
		output := stdoutBytes.String()
		output = strings.TrimRight(output, "\r\n")

		doneFn(output)
	}()

	// Wait for the command to start
	select {
	case <-ctx.Done():
		logger.Error("Command canceled", map[string]any{"error": ctx.Err()})
	case <-startedCh:
		logger.Info("Command started", map[string]any{"command": command})
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		return errors.New("command timeout")
	}

	return nil
}

func logOutput(r io.Reader, startedCh, finishedCh chan struct{}) {
	defer close(finishedCh)
	lr := linereader.New(r)

	// Wait for the command to start
	line := <-lr.Ch
	startedCh <- struct{}{}
	logger.Debug("Command output", map[string]any{"line": line})

	// Log the rest of the output
	for line := range lr.Ch {
		logger.Debug("Command output", map[string]any{"line": line})
	}
}

// isWindows checks if the current OS is Windows.
func isWindows() bool {
	return strings.HasPrefix(strings.ToLower(os.Getenv("OS")), "windows")
}
