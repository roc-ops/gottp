package returners

import (
	"bytes"
	"os"
	"testing"
)

func TestNewTerminalReturner(t *testing.T) {
	returner := NewTerminalReturner()
	
	if returner == nil {
		t.Fatal("NewTerminalReturner() returned nil")
	}
}

func TestTerminalReturner_Return(t *testing.T) {
	returner := NewTerminalReturner()
	
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "write string data",
			data:    []byte("test data"),
			wantErr: false,
		},
		{
			name:    "write empty data",
			data:    []byte(""),
			wantErr: false,
		},
		{
			name:    "write binary data",
			data:    []byte{0x00, 0x01, 0x02, 0xFF},
			wantErr: false,
		},
		{
			name:    "write multiline data",
			data:    []byte("line1\nline2\nline3"),
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			
			err := returner.Return(tt.data)
			
			// Restore stdout
			w.Close()
			os.Stdout = oldStdout
			
			if (err != nil) != tt.wantErr {
				t.Errorf("TerminalReturner.Return() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				// Read captured output
				var buf bytes.Buffer
				buf.ReadFrom(r)
				output := buf.Bytes()
				
				if !bytes.Equal(output, tt.data) {
					t.Errorf("TerminalReturner.Return() output = %v, want %v", output, tt.data)
				}
			}
		})
	}
}

func TestTerminalReturner_ReturnString(t *testing.T) {
	returner := NewTerminalReturner()
	
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "write string",
			data:    "test string",
			wantErr: false,
		},
		{
			name:    "write empty string",
			data:    "",
			wantErr: false,
		},
		{
			name:    "write multiline string",
			data:    "line1\nline2\nline3",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			
			err := returner.ReturnString(tt.data)
			
			// Restore stdout
			w.Close()
			os.Stdout = oldStdout
			
			if (err != nil) != tt.wantErr {
				t.Errorf("TerminalReturner.ReturnString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				// Read captured output
				var buf bytes.Buffer
				buf.ReadFrom(r)
				output := buf.String()
				
				if output != tt.data {
					t.Errorf("TerminalReturner.ReturnString() output = %v, want %v", output, tt.data)
				}
			}
		})
	}
}

