// +build !python

package macro

import "fmt"

// PythonEngineStub provides a stub implementation when Python support is not enabled
type PythonEngineStub struct{}

// NewPythonEngineStub creates a stub Python engine
func NewPythonEngineStub() *PythonEngineStub {
	return &PythonEngineStub{}
}

// RegisterMacro returns an error indicating Python support is not enabled
func (e *PythonEngineStub) RegisterMacro(name, source string) error {
	return fmt.Errorf("Python macro support is not enabled. Build with -tags python and ensure Python development headers are installed")
}

// RegisterMacroSource returns an error indicating Python support is not enabled
func (e *PythonEngineStub) RegisterMacroSource(source string) error {
	return fmt.Errorf("Python macro support is not enabled. Build with -tags python and ensure Python development headers are installed")
}

// ExecuteMacro returns an error indicating Python support is not enabled
func (e *PythonEngineStub) ExecuteMacro(name string, data interface{}, ttpContext map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("Python macro support is not enabled. Build with -tags python and ensure Python development headers are installed")
}

// SetTTPContext is a no-op for the stub
func (e *PythonEngineStub) SetTTPContext(context map[string]interface{}) {
	// No-op
}

