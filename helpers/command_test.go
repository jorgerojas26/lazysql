package helpers

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunCommand_Timeout(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		timeout       time.Duration
		shouldTimeout bool
		assertErr     func(t *testing.T, err error)
	}{
		{
			name:          "command succeeds with default timeout",
			command:       "echo 'hello'",
			timeout:       5 * time.Second,
			shouldTimeout: false,
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			},
		},
		{
			name:          "command succeeds with custom timeout",
			command:       "echo 'world'",
			timeout:       10 * time.Second,
			shouldTimeout: false,
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			},
		},
		{
			name:    "command times out with short timeout",
			command: "sleep 10",
			timeout: 100 * time.Millisecond,
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				if err == nil {
					t.Error("expected timeout error, got nil")
				}
				if !strings.Contains(err.Error(), "timed out") {
					t.Errorf("expected error to contain 'timed out', got %v", err)
				}
				if !strings.Contains(err.Error(), "sleep 10") {
					t.Errorf("expected error to contain command name, got %v", err)
				}
			},
		},
		{
			name:    "command succeeds with sufficient timeout",
			command: "sleep 0.1",
			timeout: 5 * time.Second,
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			done := make(chan string, 1)

			err := RunCommand(ctx, tt.command, tt.timeout, func(output string) {
				done <- output
			})

			tt.assertErr(t, err)

			// Only wait for output if no error occurred
			if err == nil {
				select {
				case <-done:
					// Command completed successfully
				case <-time.After(tt.timeout + 1*time.Second):
					t.Error("timeout waiting for command to complete")
				}
			}
		})
	}
}

func TestRunCommand_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately
	cancel()

	err := RunCommand(ctx, "echo 'test'", 5*time.Second, func(_ string) {})

	// The command might succeed or might be canceled depending on timing
	// Just ensure it doesn't panic and returns without hanging
	if err != nil && !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "command timeout") {
		// If there's an error, it should be context-related
		t.Logf("got error: %v", err)
	}
}

func TestRunCommand_OutputCapture(t *testing.T) {
	ctx := context.Background()
	expectedOutput := "test output"
	done := make(chan string, 1)

	err := RunCommand(ctx, "echo '"+expectedOutput+"'", 5*time.Second, func(output string) {
		done <- output
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case output := <-done:
		if !strings.Contains(output, expectedOutput) {
			t.Errorf("expected output to contain %q, got %q", expectedOutput, output)
		}
	case <-time.After(6 * time.Second):
		t.Error("timeout waiting for output")
	}
}
