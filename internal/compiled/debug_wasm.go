//go:build js && wasm

package compiled

import (
	"syscall/js"
)

// useConsoleLog logs to browser console (WASM only)
func useConsoleLog(msg string) {
	// Get console object and call log method
	// This will work in WASM builds where syscall/js is available
	console := js.Global().Get("console")
	if !console.IsUndefined() {
		console.Call("log", msg)
	}
}

