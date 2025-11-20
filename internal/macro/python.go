// +build python

package macro

import (
	"fmt"
	"sync"
	"unsafe"
)

/*
#cgo pkg-config: python3
#include <Python.h>
#include <stdlib.h>

// Helper function to convert Go string to Python string
PyObject* PyString_FromGoString(const char* str) {
    return PyUnicode_FromString(str);
}

// Helper function to convert Python string to Go string
char* PyString_AsGoString(PyObject* obj) {
    if (!PyUnicode_Check(obj)) {
        return NULL;
    }
    PyObject* bytes = PyUnicode_AsUTF8String(obj);
    if (!bytes) {
        return NULL;
    }
    char* result = PyBytes_AsString(bytes);
    if (!result) {
        Py_DECREF(bytes);
        return NULL;
    }
    // Note: Caller must free this
    char* copy = strdup(result);
    Py_DECREF(bytes);
    return copy;
}
*/
import "C"

// PythonEngine executes Python macros using CGO
type PythonEngine struct {
	mu         sync.RWMutex
	macros     map[string]string // macro name -> source code
	ttpContext map[string]interface{}
	initialized bool
}

// initialize initializes the Python interpreter
func (e *PythonEngine) initialize() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.initialized {
		return
	}

	// Initialize Python interpreter
	C.Py_Initialize()
	if !C.Py_IsInitialized() {
		panic("failed to initialize Python interpreter")
	}

	e.initialized = true
}

// RegisterMacro registers a Python macro function
func (e *PythonEngine) RegisterMacro(name, source string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate the Python code
	err := e.validatePython(source)
	if err != nil {
		return fmt.Errorf("failed to validate Python macro %s: %w", name, err)
	}

	// Store the source
	e.macros[name] = source
	return nil
}

// RegisterMacroSource registers macro source code directly
func (e *PythonEngine) RegisterMacroSource(source string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate the source
	err := e.validatePython(source)
	if err != nil {
		return fmt.Errorf("failed to validate Python macro source: %w", err)
	}

	// Store source
	e.macros["_source_"] = source
	return nil
}

// validatePython validates Python code by compiling it
func (e *PythonEngine) validatePython(source string) error {
	cSource := C.CString(source)
	defer C.free(unsafe.Pointer(cSource))

	// Compile the code
	pyCode := C.Py_CompileString(cSource, C.CString("<macro>"), C.Py_file_input)
	if pyCode == nil {
		// Get error
		if C.PyErr_Occurred() != nil {
			C.PyErr_Print()
			return fmt.Errorf("Python compilation error")
		}
		return fmt.Errorf("failed to compile Python code")
	}
	C.Py_DECREF(pyCode)

	return nil
}

// ExecuteMacro executes a Python macro function
func (e *PythonEngine) ExecuteMacro(name string, data interface{}, ttpContext map[string]interface{}) (interface{}, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Find macro source
	var source string
	var found bool

	if s, ok := e.macros[name]; ok {
		source = s
		found = true
	} else if s, ok := e.macros["_source_"]; ok {
		source = s
		found = true
	}

	if !found {
		return nil, fmt.Errorf("macro %s not found", name)
	}

	// Execute the macro
	return e.executePythonMacro(name, source, data, ttpContext)
}

// executePythonMacro executes Python code and calls the macro function
func (e *PythonEngine) executePythonMacro(name, source string, data interface{}, ttpContext map[string]interface{}) (interface{}, error) {
	// Convert data to Python object
	pyData, err := e.goToPython(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert data to Python: %w", err)
	}
	defer C.Py_DECREF(pyData)

	// Set up _ttp_ context if provided
	if ttpContext != nil || e.ttpContext != nil {
		ctx := ttpContext
		if ctx == nil {
			ctx = e.ttpContext
		}
		pyContext, err := e.goToPython(ctx)
		if err == nil {
			// Set _ttp_ in globals
			pyGlobals := C.PyDict_New()
			C.PyDict_SetItemString(pyGlobals, C.CString("_ttp_"), pyContext)
			C.Py_DECREF(pyContext)
			// Note: We'd need to merge with existing globals in a real implementation
			_ = pyGlobals
		}
	}

	// Execute the source code
	cSource := C.CString(source)
	defer C.free(unsafe.Pointer(cSource))

	pyCode := C.Py_CompileString(cSource, C.CString("<macro>"), C.Py_file_input)
	if pyCode == nil {
		if C.PyErr_Occurred() != nil {
			C.PyErr_Print()
		}
		return nil, fmt.Errorf("failed to compile Python macro")
	}
	defer C.Py_DECREF(pyCode)

	pyModule := C.PyImport_ExecCodeModule(C.CString("_macro_module"), pyCode)
	if pyModule == nil {
		if C.PyErr_Occurred() != nil {
			C.PyErr_Print()
		}
		return nil, fmt.Errorf("failed to execute Python macro")
	}
	defer C.Py_DECREF(pyModule)

	// Get the function
	pyFunc := C.PyObject_GetAttrString(pyModule, C.CString(name))
	if pyFunc == nil || !C.PyCallable_Check(pyFunc) {
		if pyFunc != nil {
			C.Py_DECREF(pyFunc)
		}
		return nil, fmt.Errorf("function %s not found or not callable", name)
	}
	defer C.Py_DECREF(pyFunc)

	// Call the function
	pyArgs := C.PyTuple_New(1)
	C.PyTuple_SetItem(pyArgs, 0, pyData)
	C.Py_INCREF(pyData) // Tuple takes ownership

	pyResult := C.PyObject_CallObject(pyFunc, pyArgs)
	C.Py_DECREF(pyArgs)

	if pyResult == nil {
		if C.PyErr_Occurred() != nil {
			C.PyErr_Print()
		}
		return nil, fmt.Errorf("failed to call macro function %s", name)
	}
	defer C.Py_DECREF(pyResult)

	// Convert result back to Go
	return e.pythonToGo(pyResult)
}

// goToPython converts a Go value to a Python object
func (e *PythonEngine) goToPython(v interface{}) (*C.PyObject, error) {
	switch val := v.(type) {
	case string:
		cStr := C.CString(val)
		defer C.free(unsafe.Pointer(cStr))
		return C.PyString_FromGoString(cStr), nil
	case int:
		return C.PyLong_FromLong(C.long(val)), nil
	case int64:
		return C.PyLong_FromLongLong(C.longlong(val)), nil
	case float64:
		return C.PyFloat_FromDouble(C.double(val)), nil
	case bool:
		if val {
			return C.Py_True, nil
		}
		return C.Py_False, nil
	case []interface{}:
		pyList := C.PyList_New(C.Py_ssize_t(len(val)))
		for i, item := range val {
			pyItem, err := e.goToPython(item)
			if err != nil {
				C.Py_DECREF(pyList)
				return nil, err
			}
			C.PyList_SetItem(pyList, C.Py_ssize_t(i), pyItem)
		}
		return pyList, nil
	case map[string]interface{}:
		pyDict := C.PyDict_New()
		for k, v := range val {
			pyKey, err := e.goToPython(k)
			if err != nil {
				C.Py_DECREF(pyDict)
				return nil, err
			}
			pyValue, err := e.goToPython(v)
			if err != nil {
				C.Py_DECREF(pyKey)
				C.Py_DECREF(pyDict)
				return nil, err
			}
			C.PyDict_SetItem(pyDict, pyKey, pyValue)
			C.Py_DECREF(pyKey)
			C.Py_DECREF(pyValue)
		}
		return pyDict, nil
	case nil:
		return C.Py_None, nil
	default:
		// Convert to string as fallback
		str := fmt.Sprintf("%v", v)
		return e.goToPython(str)
	}
}

// pythonToGo converts a Python object to a Go value
func (e *PythonEngine) pythonToGo(obj *C.PyObject) (interface{}, error) {
	if obj == nil {
		return nil, fmt.Errorf("nil Python object")
	}

	// Handle None
	if obj == C.Py_None {
		return nil, nil
	}

	// Handle bool
	if C.PyBool_Check(obj) != 0 {
		return obj == C.Py_True, nil
	}

	// Handle int
	if C.PyLong_Check(obj) != 0 {
		return int64(C.PyLong_AsLongLong(obj)), nil
	}

	// Handle float
	if C.PyFloat_Check(obj) != 0 {
		return float64(C.PyFloat_AsDouble(obj)), nil
	}

	// Handle string
	if C.PyUnicode_Check(obj) != 0 {
		cStr := C.PyString_AsGoString(obj)
		if cStr == nil {
			return nil, fmt.Errorf("failed to convert Python string")
		}
		defer C.free(unsafe.Pointer(cStr))
		return C.GoString(cStr), nil
	}

	// Handle list
	if C.PyList_Check(obj) != 0 {
		length := int(C.PyList_Size(obj))
		result := make([]interface{}, length)
		for i := 0; i < length; i++ {
			item := C.PyList_GetItem(obj, C.Py_ssize_t(i))
			C.Py_INCREF(item)
			value, err := e.pythonToGo(item)
			C.Py_DECREF(item)
			if err != nil {
				return nil, err
			}
			result[i] = value
		}
		return result, nil
	}

	// Handle dict
	if C.PyDict_Check(obj) != 0 {
		result := make(map[string]interface{})
		pyKeys := C.PyDict_Keys(obj)
		defer C.Py_DECREF(pyKeys)
		length := int(C.PyList_Size(pyKeys))
		for i := 0; i < length; i++ {
			pyKey := C.PyList_GetItem(pyKeys, C.Py_ssize_t(i))
			C.Py_INCREF(pyKey)
			pyValue := C.PyDict_GetItem(obj, pyKey)
			C.Py_INCREF(pyValue)

			key, err := e.pythonToGo(pyKey)
			C.Py_DECREF(pyKey)
			if err != nil {
				C.Py_DECREF(pyValue)
				return nil, err
			}

			value, err := e.pythonToGo(pyValue)
			C.Py_DECREF(pyValue)
			if err != nil {
				return nil, err
			}

			if keyStr, ok := key.(string); ok {
				result[keyStr] = value
			}
		}
		return result, nil
	}

	// Fallback: convert to string
	pyStr := C.PyObject_Str(obj)
	if pyStr != nil {
		defer C.Py_DECREF(pyStr)
		cStr := C.PyString_AsGoString(pyStr)
		if cStr != nil {
			defer C.free(unsafe.Pointer(cStr))
			return C.GoString(cStr), nil
		}
	}

	return nil, fmt.Errorf("unsupported Python type")
}

// SetTTPContext sets the _ttp_ context dictionary
func (e *PythonEngine) SetTTPContext(context map[string]interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ttpContext = context
}

// Close shuts down the Python interpreter
func (e *PythonEngine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.initialized {
		C.Py_Finalize()
		e.initialized = false
	}
}

