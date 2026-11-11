//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func fakeBrowserLauncher(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("Linux uses xdg-open")
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "xdg-open"), []byte("#!/bin/sh\n"+script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
}

func TestBrowserLauncherFailure(t *testing.T) {
	fakeBrowserLauncher(t, "exit 7\n")
	if err := runControlWindow("http://127.0.0.1:7070/", make(chan struct{})); err == nil {
		t.Fatal("failed browser launch should be reported")
	}
}

func TestBrowserTabDoesNotOwnApplicationLifetime(t *testing.T) {
	fakeBrowserLauncher(t, "exit 0\n")
	shutdown := make(chan struct{})
	result := make(chan error, 1)
	go func() { result <- runControlWindow("http://127.0.0.1:7070/", shutdown) }()
	select {
	case err := <-result:
		t.Fatalf("application stopped when launcher exited: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(shutdown)
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("application did not respond to shutdown")
	}
}

func TestShutdownWhileBrowserLauncherIsRunning(t *testing.T) {
	fakeBrowserLauncher(t, "/bin/sleep 2\n")
	shutdown := make(chan struct{})
	close(shutdown)
	result := make(chan error, 1)
	go func() { result <- runControlWindow("http://127.0.0.1:7070/", shutdown) }()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown waited for the external browser process")
	}
}
