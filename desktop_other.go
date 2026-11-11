//go:build !windows

package main

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
)

func runControlWindow(address string, shutdown <-chan struct{}) error {
	var command *exec.Cmd
	if runtime.GOOS == "darwin" {
		command = exec.Command("open", address)
	} else {
		command = exec.Command("xdg-open", address)
	}
	// Reap the browser launcher and report failures (for example, no browser
	// configured). The browser itself normally detaches from this command.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := command.Start(); err != nil {
		return err
	}
	launched := make(chan error, 1)
	go func() { launched <- command.Wait() }()
	if shutdown != nil {
		for {
			select {
			case err := <-launched:
				if err != nil {
					return err
				}
				launched = nil
			case <-shutdown:
				return nil
			case <-ctx.Done():
				return nil
			}
		}
	}
	return nil
}

func showControlWindowError(error) {}
