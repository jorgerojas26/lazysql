package helpers

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/mitchellh/go-linereader"

	"github.com/jorgerojas26/lazysql/helpers/logger"
)

func RunCommand(ctx context.Context, command string, doneFn func()) error {
	var cmd *exec.Cmd

	parts := strings.Fields(command)
	if len(parts) == 1 {
		cmd = exec.CommandContext(ctx, parts[0]) // #nosec G204
	} else {
		cmd = exec.CommandContext(ctx, parts[0], parts[1:]...) // #nosec G204
	}

	// Create a pipe to read the output from.
	pr, pw := io.Pipe()
	startedCh := make(chan struct{})
	finishedCh := make(chan struct{})
	go logOutput(pr, startedCh, finishedCh)

	// Connect the pipe to stdout and stderr.
	cmd.Stderr = pw
	cmd.Stdout = pw

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			logger.Error("Command stopped", map[string]any{"error": err.Error()})
		}

		_ = pw.Close()
		<-finishedCh
		doneFn()
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
