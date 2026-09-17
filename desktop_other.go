//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

func runControlWindow(address string, shutdown <-chan struct{}) error {
	var command *exec.Cmd
	if runtime.GOOS == "darwin" {
		command = exec.Command("open", address)
	} else {
		command = exec.Command("xdg-open", address)
	}
	if err := command.Start(); err != nil {
		return err
	}
	if shutdown != nil {
		<-shutdown
	}
	return nil
}

func showControlWindowError(error) {}
