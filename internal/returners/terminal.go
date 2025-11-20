package returners

import (
	"fmt"
	"os"
)

// TerminalReturner returns results to terminal/stdout
type TerminalReturner struct{}

// NewTerminalReturner creates a new terminal returner
func NewTerminalReturner() *TerminalReturner {
	return &TerminalReturner{}
}

// Return writes data to stdout
func (t *TerminalReturner) Return(data []byte) error {
	_, err := os.Stdout.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to terminal: %w", err)
	}
	return nil
}

// ReturnString writes string data to stdout
func (t *TerminalReturner) ReturnString(data string) error {
	_, err := fmt.Print(data)
	return err
}

