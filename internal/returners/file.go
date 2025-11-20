package returners

import (
	"fmt"
	"os"
)

// FileReturner returns results to a file
type FileReturner struct {
	Path string
}

// NewFileReturner creates a new file returner
func NewFileReturner(path string) *FileReturner {
	return &FileReturner{
		Path: path,
	}
}

// Return writes data to file
func (f *FileReturner) Return(data []byte) error {
	if f.Path == "" {
		return fmt.Errorf("file path not specified")
	}
	
	err := os.WriteFile(f.Path, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", f.Path, err)
	}
	return nil
}

// ReturnString writes string data to file
func (f *FileReturner) ReturnString(data string) error {
	return f.Return([]byte(data))
}

