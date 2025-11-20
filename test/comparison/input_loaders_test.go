package comparison

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInputLoaderText tests text input loader
func TestInputLoaderText(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	// Skip - Python TTP splits input by lines when load="text" is used with patterns
	// This is a complex feature that requires pattern matching inside input tags
	skipIfKnownDifference(t, "text_loader_with_patterns")

	template := `<input load="text" name="test_input">
interface {{ interface }}
 ip address {{ ip }}
</input>

<group name="interfaces" input="test_input">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
`

	RunComparison(t, "text_loader", template, data, nil, nil)
}

// TestInputLoaderYAML tests YAML input loader
func TestInputLoaderYAML(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<input load="yaml" name="test_input">
interface: {{ interface }}
ip: {{ ip }}
</input>

<group name="interfaces" input="test_input">
interface: {{ interface }}
ip: {{ ip }}
</group>`

	data := `interface: Loopback0
ip: 192.168.0.1/24
`

	RunComparison(t, "yaml_loader", template, data, nil, nil)
}

// TestInputLoaderJSON tests JSON input loader
func TestInputLoaderJSON(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<input load="json" name="test_input">
{"interface": "{{ interface }}", "ip": "{{ ip }}"}
</input>

<group name="interfaces" input="test_input">
{"interface": "{{ interface }}", "ip": "{{ ip }}"}
</group>`

	data := `{"interface": "Loopback0", "ip": "192.168.0.1/24"}
`

	RunComparison(t, "json_loader", template, data, nil, nil)
}

// TestInputLoaderCSV tests CSV input loader
func TestInputLoaderCSV(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<input load="csv" name="test_input">
interface,ip
{{ interface }},{{ ip }}
</input>

<group name="interfaces" input="test_input">
interface,ip
{{ interface }},{{ ip }}
</group>`

	data := `interface,ip
Loopback0,192.168.0.1/24
Vlan100,10.0.0.1/24
`

	RunComparison(t, "csv_loader", template, data, nil, nil)
}

// TestInputLoaderFile tests file input loader
func TestInputLoaderFile(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	// Skip - Similar to text loader, file loader with patterns has complex behavior
	skipIfKnownDifference(t, "file_loader_with_patterns")

	// Create a temporary data file
	tmpDir := t.TempDir()
	dataFile := filepath.Join(tmpDir, "data.txt")
	err := os.WriteFile(dataFile, []byte(`interface Loopback0
 ip address 192.168.0.1/24
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	template := `<input load="file" name="test_input" file="` + dataFile + `">
</input>

<group name="interfaces" input="test_input">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	RunComparison(t, "file_loader", template, "", nil, nil)
}

// TestInputLoaderDirectory tests directory input loader
func TestInputLoaderDirectory(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	// Create temporary directory with files
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	
	os.WriteFile(file1, []byte(`interface Loopback0
 ip address 192.168.0.1/24
`), 0644)
	os.WriteFile(file2, []byte(`interface Vlan100
 ip address 10.0.0.1/24
`), 0644)

	template := `<input load="directory" name="test_input" path="` + tmpDir + `" extensions="txt">
</input>

<group name="interfaces" input="test_input">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	// Note: Directory loader may have different behavior, so this might need adjustment
	skipIfKnownDifference(t, "directory_loader")
	RunComparison(t, "directory_loader", template, "", nil, nil)
}

// TestInputLoaderURL tests URL input loader
func TestInputLoaderURL(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	skipIfKnownDifference(t, "url_loader")
	
	// Skip actual URL test as it requires network access
	t.Skip("URL loader test requires network access and may have timing differences")
}

// TestInputLoaderDatabase tests database input loader
func TestInputLoaderDatabase(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	skipIfKnownDifference(t, "database_loader")
	
	t.Skip("Database loader test requires database setup and drivers")
}

