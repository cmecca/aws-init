//go:build !windows
// Package main provides process execution and signal handling functionality.
//
// This file contains functions for executing child processes with proper
// signal forwarding, process group management, and graceful shutdown handling.
//
// # Signal Handling
//
// The executor forwards these signals to child processes:
//   - SIGTERM, SIGINT, SIGQUIT (termination signals)
//   - SIGUSR1, SIGUSR2 (user-defined signals)
//
// # Graceful Shutdown
//
// When SIGTERM is received:
//  1. Forward SIGTERM to child process and process group
//  2. Wait up to 10 seconds for graceful shutdown
//  3. Send SIGKILL if process hasn't exited
//
// # Process Groups
//
// Child processes are started in their own process group to ensure
// proper signal propagation to all descendants.
package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

const gracefulTimeout = 10 * time.Second

// execute runs a command with proper signal handling and process group management.
//
// The command is started in its own process group to ensure proper signal propagation.
// All signals are forwarded to the child process, with special handling for SIGTERM
// which triggers a graceful shutdown sequence.
//
// Parameters:
//   - command: the executable to run
//   - args: command line arguments
//   - env: environment variables for the process
//
// Returns the exit code of the child process, or 1 if execution fails.
//
// Exit codes:
//   - 0: successful execution
//   - 1: execution failed or process start error
//   - other: exit code from child process
func execute(command string, args []string, env []string) int {
	cmd := exec.Command(command, args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		log.Printf("failed to start %s: %v", command, err)
		return 1
	}

	if cmd.Process == nil {
		log.Printf("no process information available")
		return 1
	}

	pid := cmd.Process.Pid
	log.Printf("started %s (PID %d)", command, pid)

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)

	// Register signals we want to handle and forward to child process
	signal.Notify(sigChan,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
	)

	// Start signal handler
	go handleSignals(sigChan, pid)

	// Wait for process to complete
	err := cmd.Wait()

	// Stop signal notifications
	signal.Stop(sigChan)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus()
			}
		}
		log.Printf("process failed: %v", err)
		return 1
	}

	return 0
}

// handleSignals manages signal forwarding and graceful shutdown for child processes.
//
// This function runs in a separate goroutine and forwards received signals to the
// child process and its process group. For SIGTERM, it implements a graceful
// shutdown with a 10-second timeout before force-killing the process.
//
// Handled signals:
//   - SIGTERM, SIGINT, SIGQUIT: forwarded with graceful shutdown for SIGTERM
//   - SIGUSR1, SIGUSR2: forwarded directly
//   - others: ignored with log message
//
// The sigChan should be closed by the caller when signal handling is no longer needed.
func handleSignals(sigChan chan os.Signal, pid int) {
	for sig := range sigChan {
		switch sig {
		case syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT:
			log.Printf("forwarding signal %v to PID %d", sig, pid)
			forwardSignal(pid, sig)

			if sig == syscall.SIGTERM {
				go func() {
					time.Sleep(gracefulTimeout)
					log.Printf("graceful timeout expired, force killing PID %d", pid)
					if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
						log.Printf("failed to SIGKILL PID %d: %v", pid, err)
					}
					if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
						log.Printf("failed to SIGKILL group -%d: %v", pid, err)
					}
				}()
			}

		case syscall.SIGUSR1, syscall.SIGUSR2:
			log.Printf("forwarding signal %v to PID %d", sig, pid)
			forwardSignal(pid, sig)

		default:
			log.Printf("ignoring signal %v", sig)
		}
	}
}

// forwardSignal sends a signal to both a process and its process group.
//
// This ensures that signals reach both the direct child process and any
// descendant processes in the same process group. Errors are logged but
// do not stop execution.
//
// Parameters:
//   - pid: process ID of the target process
//   - sig: signal to send (must be a syscall.Signal)
func forwardSignal(pid int, sig os.Signal) {
	syscallSig, ok := sig.(syscall.Signal)
	if !ok {
		// Handle the case where sig is not a syscall.Signal
		// You might want to log this or handle it appropriately for your use case
		return // or handle appropriately
	}

	// Send to process group (negative PID)
	if err := syscall.Kill(-pid, syscallSig); err != nil {
		log.Printf("failed to signal group -%d: %v", pid, err)
	}

	// Send to process directly
	if err := syscall.Kill(pid, syscallSig); err != nil {
		log.Printf("failed to signal PID %d: %v", pid, err)
	}
}
