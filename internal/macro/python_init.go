// +build python

package macro

// initPythonEngine initializes the Python engine when Python support is enabled
func (r *MacroRegistry) initPythonEngine() {
	r.python = NewPythonEngine()
}

