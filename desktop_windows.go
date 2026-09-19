//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

const (
	messageBoxOK        = 0x00000000
	messageBoxIconError = 0x00000010
)

var messageBoxW = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")

func runControlWindow(address string, shutdown <-chan struct{}) error {
	cacheDirectory, err := os.UserCacheDir()
	if err != nil {
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	window := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(cacheDirectory, productName, "WebView2"),
		WindowOptions: webview2.WindowOptions{
			Title:  productName,
			Width:  900,
			Height: 760,
			IconId: 1,
			Center: true,
		},
	})
	if window == nil {
		return errors.New("WebView2 could not create the application window")
	}
	defer window.Destroy()

	windowClosed := make(chan struct{})
	defer close(windowClosed)
	if shutdown != nil {
		go func() {
			select {
			case <-shutdown:
				window.Dispatch(window.Terminate)
			case <-windowClosed:
			}
		}()
	}

	window.SetSize(720, 620, webview2.HintMin)
	window.Navigate(address)
	window.Run()
	return nil
}

func showControlWindowError(err error) {
	message, _ := windows.UTF16PtrFromString("The desktop control window could not open. Make sure the Microsoft Edge WebView2 Runtime is installed, then try again.\n\n" + err.Error())
	title, _ := windows.UTF16PtrFromString(productName)
	_, _, _ = messageBoxW.Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), messageBoxOK|messageBoxIconError)
}

// applyUpdate replaces the running executable with a verified download and
// relaunches it. Windows keeps the running image locked, so the current file is
// renamed aside first; renaming is permitted where overwriting is not.
func applyUpdate(staged string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate the running application: %w", err)
	}
	previous := executable + ".old"
	_ = os.Remove(previous)
	if err := os.Rename(executable, previous); err != nil {
		return fmt.Errorf("move the running application aside: %w", err)
	}
	if err := os.Rename(staged, executable); err != nil {
		_ = os.Rename(previous, executable)
		return fmt.Errorf("install the downloaded update: %w", err)
	}
	command := exec.Command(executable)
	command.Dir = filepath.Dir(executable)
	if err := command.Start(); err != nil {
		return fmt.Errorf("restart the application: %w", err)
	}
	return nil
}

// cleanupPreviousUpdate removes the renamed previous executable left behind by
// an earlier update. It is best effort because the file may still be locked.
func cleanupPreviousUpdate() {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	_ = os.Remove(executable + ".old")
}
