//go:build windows

package main

import (
	"errors"
	"os"
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
