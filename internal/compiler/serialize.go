package compiler

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// SerializationFormat represents the format for serialization
type SerializationFormat string

const (
	FormatGob  SerializationFormat = "gob"
	FormatJSON SerializationFormat = "json"
	FormatYAML SerializationFormat = "yaml"
)

// SaveCompiledTemplate saves a compiled template to a writer
func SaveCompiledTemplate(compiled *CompiledTemplate, w io.Writer, format SerializationFormat) error {
	switch format {
	case FormatGob:
		encoder := gob.NewEncoder(w)
		return encoder.Encode(compiled)
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(compiled)
	case FormatYAML:
		encoder := yaml.NewEncoder(w)
		defer encoder.Close()
		return encoder.Encode(compiled)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// LoadCompiledTemplate loads a compiled template from a reader
func LoadCompiledTemplate(r io.Reader, format SerializationFormat) (*CompiledTemplate, error) {
	compiled := &CompiledTemplate{}

	switch format {
	case FormatGob:
		decoder := gob.NewDecoder(r)
		if err := decoder.Decode(compiled); err != nil {
			return nil, fmt.Errorf("failed to decode gob: %w", err)
		}
	case FormatJSON:
		decoder := json.NewDecoder(r)
		if err := decoder.Decode(compiled); err != nil {
			return nil, fmt.Errorf("failed to decode JSON: %w", err)
		}
	case FormatYAML:
		decoder := yaml.NewDecoder(r)
		if err := decoder.Decode(compiled); err != nil {
			return nil, fmt.Errorf("failed to decode YAML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	return compiled, nil
}

// SaveCompiledTemplateToBytes saves a compiled template to bytes
func SaveCompiledTemplateToBytes(compiled *CompiledTemplate, format SerializationFormat) ([]byte, error) {
	var buf bytes.Buffer
	if err := SaveCompiledTemplate(compiled, &buf, format); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// LoadCompiledTemplateFromBytes loads a compiled template from bytes
func LoadCompiledTemplateFromBytes(data []byte, format SerializationFormat) (*CompiledTemplate, error) {
	return LoadCompiledTemplate(bytes.NewReader(data), format)
}

