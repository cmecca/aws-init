package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestExecute(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping executor tests in short mode")
	}

	tests := []struct {
		name     string
		command  string
		args     []string
		env      []string
		wantCode int
	}{
		{
			name:     "successful echo command",
			command:  "echo",
			args:     []string{"hello", "world"},
			env:      []string{"PATH=/usr/bin:/bin"},
			wantCode: 0,
		},
		{
			name:     "successful true command",
			command:  "true",
			args:     []string{},
			env:      []string{"PATH=/usr/bin:/bin"},
			wantCode: 0,
		},
		{
			name:     "failing false command",
			command:  "false",
			args:     []string{},
			env:      []string{"PATH=/usr/bin:/bin"},
			wantCode: 1,
		},
		{
			name:     "nonexistent command",
			command:  "nonexistent-command-12345",
			args:     []string{},
			env:      []string{"PATH=/usr/bin:/bin"},
			wantCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && (tt.command == "true" || tt.command == "false") {
				t.Skip("skipping unix command test on windows")
			}

			code := execute(tt.command, tt.args, tt.env)
			if code != tt.wantCode {
				t.Errorf("execute() = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

func TestExecuteWithCustomEnv(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping executor tests in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("skipping shell command test on windows")
	}

	// Test that custom environment is properly passed
	customEnv := []string{
		"TEST_VAR=custom_value",
		"PATH=/usr/bin:/bin",
	}

	// Use shell to check environment variable
	code := execute("sh", []string{"-c", "[ \"$TEST_VAR\" = \"custom_value\" ]"}, customEnv)
	if code != 0 {
		t.Error("custom environment variable was not set correctly")
	}
}

func TestExecuteExitCodes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping executor tests in short mode")
	}

	tests := []struct {
		name     string
		exitCode int
	}{
		{"exit 0", 0},
		{"exit 1", 1},
		{"exit 42", 42},
		{"exit 127", 127},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var command string
			var args []string

			if runtime.GOOS == "windows" {
				command = "cmd"
				args = []string{"/c", fmt.Sprintf("exit %d", tt.exitCode)}
			} else {
				command = "sh"
				args = []string{"-c", fmt.Sprintf("exit %d", tt.exitCode)}
			}

			code := execute(command, args, []string{"PATH=/usr/bin:/bin"})
			if code != tt.exitCode {
				t.Errorf("execute() = %d, want %d", code, tt.exitCode)
			}
		})
	}
}

func TestExecuteWithLongRunningProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running process test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("skipping signal test on windows")
	}

	// This test verifies that the process execution doesn't hang
	// We run a command that should complete quickly
	start := time.Now()
	code := execute("sleep", []string{"0.1"}, []string{"PATH=/usr/bin:/bin"})
	duration := time.Since(start)

	if code != 0 {
		t.Errorf("sleep command failed with code %d", code)
	}

	// Should complete in reasonable time (much less than 1 second)
	if duration > 1*time.Second {
		t.Errorf("execution took too long: %v", duration)
	}
}

func TestForwardSignal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping signal test on windows")
	}

	// Test that forwardSignal doesn't panic or cause issues
	// when called with current process PID (safe test)
	currentPID := os.Getpid()

	// These calls should not cause any issues
	forwardSignal(currentPID, os.Signal(syscall.SIGUSR1))
	forwardSignal(currentPID, os.Signal(syscall.SIGUSR2))

	// Test with an obviously invalid PID should handle errors gracefully
	forwardSignal(999999, os.Signal(syscall.SIGUSR1))
}

func TestExecuteProcessGroupHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping process group test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("skipping process group test on windows")
	}

	// Test that we can execute a command that spawns child processes
	// This verifies that process group handling works correctly
	script := `
		echo "parent process"
		sleep 0.1 &
		wait
		echo "done"
	`

	start := time.Now()
	code := execute("sh", []string{"-c", script}, []string{"PATH=/usr/bin:/bin"})
	duration := time.Since(start)

	if code != 0 {
		t.Errorf("script execution failed with code %d", code)
	}

	// Should complete in reasonable time
	if duration > 2*time.Second {
		t.Errorf("script took too long: %v", duration)
	}
}
