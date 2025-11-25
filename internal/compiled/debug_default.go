//go:build !js || !wasm

package compiled

// useConsoleLog is a no-op for non-WASM builds
func useConsoleLog(msg string) {
	// No-op for non-WASM builds
}

