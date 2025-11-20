package comparison

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/roc-ops/gottp"
)

// BenchmarkGoTTPCompile benchmarks GoTTP compilation
func BenchmarkGoTTPCompile(b *testing.B) {
	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
 description {{ description | default("") }}
</group>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gottp.CompileTemplate(template)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGoTTPParse benchmarks GoTTP parsing
func BenchmarkGoTTPParse(b *testing.B) {
	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
 description {{ description | default("") }}
</group>`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		b.Fatal(err)
	}

	data := `interface Loopback0
 ip address 192.168.0.1/24
 description Management interface
interface Vlan100
 ip address 10.0.0.1/24
 description User VLAN
interface GigabitEthernet0/0
 ip address 172.16.0.1/24
interface GigabitEthernet0/1
 ip address 172.16.1.1/24
`

	inputs := gottp.Inputs{
		"Default_Input": data,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compiled.Parse(inputs, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPythonTTPCompile benchmarks Python TTP compilation
func BenchmarkPythonTTPCompile(b *testing.B) {
	if !pythonTTPAvailable() {
		b.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
 description {{ description | default("") }}
</group>`

	root, err := getProjectRoot()
	if err != nil {
		b.Fatal(err)
	}

	scriptPath := fmt.Sprintf(`
import sys
import time
sys.path.insert(0, '%s/ttp-original')
from ttp import ttp

template = """%s"""

for i in range(%d):
    start = time.perf_counter()
    parser = ttp(template=template)
    end = time.perf_counter()
    print(f"{end - start:.6f}")
`, root, template, b.N)

	cmd := exec.Command("python3", "-c", scriptPath)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		b.Fatalf("Python TTP failed: %v\nOutput: %s", err, output)
	}

	// Parse output to get timing results
	// This is just for reporting - the actual benchmark is done in Python
	_ = output
}

// BenchmarkPythonTTPParse benchmarks Python TTP parsing
func BenchmarkPythonTTPParse(b *testing.B) {
	if !pythonTTPAvailable() {
		b.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
 description {{ description | default("") }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
 description Management interface
interface Vlan100
 ip address 10.0.0.1/24
 description User VLAN
interface GigabitEthernet0/0
 ip address 172.16.0.1/24
interface GigabitEthernet0/1
 ip address 172.16.1.1/24
`

	root, err := getProjectRoot()
	if err != nil {
		b.Fatal(err)
	}

	scriptPath := fmt.Sprintf(`
import sys
import time
sys.path.insert(0, '%s/ttp-original')
from ttp import ttp

template = """%s"""
data = """%s"""

# Compile once
parser = ttp(template=template)

for i in range(%d):
    parser = ttp(template=template)
    parser.add_input(data)
    start = time.perf_counter()
    parser.parse()
    end = time.perf_counter()
    print(f"{end - start:.6f}")
`, root, template, data, b.N)

	cmd := exec.Command("python3", "-c", scriptPath)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		b.Fatalf("Python TTP failed: %v\nOutput: %s", err, output)
	}

	// Parse output to get timing results
	_ = output
}

// TestPerformanceComparison runs a performance comparison and prints results
func TestPerformanceComparison(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
 description {{ description | default("") }}
</group>

<group name="vlans">
vlan {{ vlan_id }}
 name {{ vlan_name }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
 description Management interface
interface Vlan100
 ip address 10.0.0.1/24
 description User VLAN
interface GigabitEthernet0/0
 ip address 172.16.0.1/24
interface GigabitEthernet0/1
 ip address 172.16.1.1/24
vlan 100
 name Users
vlan 200
 name Servers
vlan 300
 name Management
`

	iterations := 1000

	fmt.Println("\n=== Performance Comparison: GoTTP vs Python TTP ===")
	fmt.Println()

	// Test GoTTP Compilation
	fmt.Println("1. Compilation Performance:")
	fmt.Println("   Running GoTTP compilation benchmark...")
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := gottp.CompileTemplate(template)
		if err != nil {
			t.Fatalf("GoTTP compilation failed: %v", err)
		}
	}
	goCompileTime := time.Since(start)
	goCompileAvg := goCompileTime / time.Duration(iterations)
	fmt.Printf("   GoTTP:     %v total, %v per operation (avg)\n", goCompileTime, goCompileAvg)

	// Test Python TTP Compilation
	fmt.Println("   Running Python TTP compilation benchmark...")
	root, err := getProjectRoot()
	if err != nil {
		t.Fatal(err)
	}

	script := fmt.Sprintf(`
import sys
import time
sys.path.insert(0, '%s/ttp-original')
from ttp import ttp

template = """%s"""

iterations = %d
start = time.perf_counter()
for i in range(iterations):
    parser = ttp(template=template)
end = time.perf_counter()
total = end - start
avg = total / iterations
print(f"{total:.6f}")
print(f"{avg:.9f}")
`, root, template, iterations)

	cmd := exec.Command("python3", "-c", script)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Python TTP compilation benchmark failed: %v\nOutput: %s", err, output)
	}

	// Parse Python output (two lines: total, avg)
	var pyCompileTotal, pyCompileAvg float64
	fmt.Sscanf(string(output), "%f\n%f", &pyCompileTotal, &pyCompileAvg)
	pyCompileTime := time.Duration(pyCompileTotal * float64(time.Second))
	pyCompileAvgTime := time.Duration(pyCompileAvg * float64(time.Second))
	fmt.Printf("   Python TTP: %v total, %v per operation (avg)\n", pyCompileTime, pyCompileAvgTime)

	compileSpeedup := float64(pyCompileTime) / float64(goCompileTime)
	fmt.Printf("   Speedup: %.2fx faster\n\n", compileSpeedup)

	// Test GoTTP Parsing
	fmt.Println("2. Parsing Performance:")
	fmt.Println("   Running GoTTP parsing benchmark...")
	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("GoTTP compilation failed: %v", err)
	}

	inputs := gottp.Inputs{
		"Default_Input": data,
	}

	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, err := compiled.Parse(inputs, nil, nil)
		if err != nil {
			t.Fatalf("GoTTP parsing failed: %v", err)
		}
	}
	goParseTime := time.Since(start)
	goParseAvg := goParseTime / time.Duration(iterations)
	fmt.Printf("   GoTTP:     %v total, %v per operation (avg)\n", goParseTime, goParseAvg)

	// Test Python TTP Parsing
	fmt.Println("   Running Python TTP parsing benchmark...")
	script = fmt.Sprintf(`
import sys
import time
sys.path.insert(0, '%s/ttp-original')
from ttp import ttp

template = """%s"""
data = """%s"""

iterations = %d
start = time.perf_counter()
for i in range(iterations):
    parser = ttp(template=template)
    parser.add_input(data)
    parser.parse()
end = time.perf_counter()
total = end - start
avg = total / iterations
print(f"{total:.6f}")
print(f"{avg:.9f}")
`, root, template, data, iterations)

	cmd = exec.Command("python3", "-c", script)
	cmd.Dir = root
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Python TTP parsing benchmark failed: %v\nOutput: %s", err, output)
	}

	// Parse Python output
	var pyParseTotal, pyParseAvg float64
	fmt.Sscanf(string(output), "%f\n%f", &pyParseTotal, &pyParseAvg)
	pyParseTime := time.Duration(pyParseTotal * float64(time.Second))
	pyParseAvgTime := time.Duration(pyParseAvg * float64(time.Second))
	fmt.Printf("   Python TTP: %v total, %v per operation (avg)\n", pyParseTime, pyParseAvgTime)

	parseSpeedup := float64(pyParseTime) / float64(goParseTime)
	fmt.Printf("   Speedup: %.2fx faster\n\n", parseSpeedup)

	// Summary
	fmt.Println("=== Summary ===")
	fmt.Printf("Compilation: GoTTP is %.2fx faster than Python TTP\n", compileSpeedup)
	fmt.Printf("Parsing:     GoTTP is %.2fx faster than Python TTP\n", parseSpeedup)
	// Geometric mean: sqrt(a * b)
	overallSpeedup := (compileSpeedup * parseSpeedup) / 2.0
	if compileSpeedup > 0 && parseSpeedup > 0 {
		overallSpeedup = (compileSpeedup + parseSpeedup) / 2.0 // Arithmetic mean for simplicity
	}
	fmt.Printf("Overall:     GoTTP is %.2fx faster (average)\n", overallSpeedup)
	fmt.Println()
}

// TestPerformanceComparisonLargeInput tests performance with a large input dataset
// This helps validate performance with many matches, especially after sorting algorithm improvements
func TestPerformanceComparisonLargeInput(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
 description {{ description | default("") }}
 shutdown {{ shutdown | default("false") }}
</group>

<group name="vlans">
vlan {{ vlan_id }}
 name {{ vlan_name }}
</group>`

	// Generate large input data (1000 interfaces + 500 vlans)
	var largeData strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&largeData, "interface GigabitEthernet0/%d\n", i)
		fmt.Fprintf(&largeData, " ip address 192.168.%d.1/24\n", i%256)
		fmt.Fprintf(&largeData, " description Interface %d\n", i)
		if i%10 == 0 {
			largeData.WriteString(" shutdown\n")
		}
		largeData.WriteString("\n")
	}
	for i := 1; i <= 500; i++ {
		fmt.Fprintf(&largeData, "vlan %d\n", i)
		fmt.Fprintf(&largeData, " name VLAN_%d\n", i)
		largeData.WriteString("\n")
	}

	data := largeData.String()
	iterations := 100 // Fewer iterations for large input

	fmt.Println("\n=== Large Input Performance Comparison: GoTTP vs Python TTP ===")
	fmt.Printf("Input size: ~%d KB (%d interfaces, %d vlans)\n", len(data)/1024, 1000, 500)
	fmt.Println()

	// Test GoTTP Parsing with Large Input
	fmt.Println("1. Large Input Parsing Performance:")
	fmt.Println("   Running GoTTP parsing benchmark...")
	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("GoTTP compilation failed: %v", err)
	}

	inputs := gottp.Inputs{
		"Default_Input": data,
	}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := compiled.Parse(inputs, nil, nil)
		if err != nil {
			t.Fatalf("GoTTP parsing failed: %v", err)
		}
	}
	goParseTime := time.Since(start)
	goParseAvg := goParseTime / time.Duration(iterations)
	fmt.Printf("   GoTTP:     %v total, %v per operation (avg)\n", goParseTime, goParseAvg)

	// Test Python TTP Parsing with Large Input
	fmt.Println("   Running Python TTP parsing benchmark...")
	root, err := getProjectRoot()
	if err != nil {
		t.Fatal(err)
	}

	// Escape data for Python string
	escapedData := strings.ReplaceAll(data, `\`, `\\`)
	escapedData = strings.ReplaceAll(escapedData, `"""`, `\"\"\"`)

	script := fmt.Sprintf(`
import sys
import time
sys.path.insert(0, '%s/ttp-original')
from ttp import ttp

template = """%s"""
data = """%s"""

iterations = %d
start = time.perf_counter()
for i in range(iterations):
    parser = ttp(template=template)
    parser.add_input(data)
    parser.parse()
end = time.perf_counter()
total = end - start
avg = total / iterations
print(f"{total:.6f}")
print(f"{avg:.9f}")
`, root, template, escapedData, iterations)

	cmd := exec.Command("python3", "-c", script)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Python TTP parsing benchmark failed: %v\nOutput: %s", err, output)
	}

	// Parse Python output
	var pyParseTotal, pyParseAvg float64
	fmt.Sscanf(string(output), "%f\n%f", &pyParseTotal, &pyParseAvg)
	pyParseTime := time.Duration(pyParseTotal * float64(time.Second))
	pyParseAvgTime := time.Duration(pyParseAvg * float64(time.Second))
	fmt.Printf("   Python TTP: %v total, %v per operation (avg)\n", pyParseTime, pyParseAvgTime)

	parseSpeedup := float64(pyParseTime) / float64(goParseTime)
	fmt.Printf("   Speedup: %.2fx faster\n\n", parseSpeedup)

	// Summary
	fmt.Println("=== Large Input Summary ===")
	fmt.Printf("Parsing:     GoTTP is %.2fx faster than Python TTP\n", parseSpeedup)
	fmt.Printf("Input size:  ~%d KB\n", len(data)/1024)
	fmt.Printf("Matches:     ~%d interface matches + %d vlan matches\n", 1000, 500)
	fmt.Println()
}

// TestPerformanceComparisonCableModem tests performance with a large cable modem dataset
// This uses real-world cable modem data to test performance with many matches
func TestPerformanceComparisonCableModem(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<macro>
def ds_bonded(data):
  if data["ds-intf"][-1] == "*":
    data["ds-intf"] = data["ds-intf"][:-1]
    data["ds-bonded"] = True
    data["ds-impaired"] = False
  elif data["ds-intf"][-1] == "#":
    data["ds-bonded"] = True
    data["ds-impaired"] = True
  else:
    data["ds-bonded"] = False
    data["ds-impaired"] = False
  return data

def us_bonded(data):
  if data["us-intf"][-1] == "*":
    data["us-intf"] = data["us-intf"][:-1]
    data["us-bonded"] = True
    data["us-impaired"] = False
  elif data["us-intf"][-1] == "#":
    data["us-bonded"] = True
    data["us-impaired"] = True
  else:
    data["us-bonded"] = False
    data["us-impaired"] = False
  return data
</macro>

<group name="show_cable_modem*" macro="ds_bonded, us_bonded">
{{mac-address | MAC | mac_eui}} {{ip-address | IP }}         {{us-intf}}      {{ds-intf }}     {{status}}     {{prim-sid}}    {{rx-power}}    {{timing-offset}}      {{num-cpes}}    {{bpi-enabled}}  {{rphy-node}}    {{mac-domain}} 
</group>`

	// Try to read input data from fixture file
	root, err := getProjectRoot()
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Try multiple possible locations for the input file
	fixturePaths := []string{
		filepath.Join(root, "test", "comparison", "fixtures", "cable_modem_input.txt"),
		"/Users/jasonpatterson/test.txt",
		filepath.Join(root, "Untitled-2"),
		"Untitled-2", // Current directory
	}

	var data string
	var dataPath string
	for _, path := range fixturePaths {
		if bytes, err := os.ReadFile(path); err == nil {
			data = string(bytes)
			dataPath = path
			break
		}
	}

	if data == "" {
		t.Skipf("Cable modem input file not found. Please place the input file at one of: %v", fixturePaths)
	}

	// Count lines for reporting
	lineCount := strings.Count(data, "\n")
	if lineCount == 0 && len(data) > 0 {
		lineCount = 1
	}

	iterations := 50 // Fewer iterations for very large input

	fmt.Println("\n=== Cable Modem Large Input Performance Comparison: GoTTP vs Python TTP ===")
	fmt.Printf("Input file:  %s\n", dataPath)
	fmt.Printf("Input size:  ~%d KB (%d lines)\n", len(data)/1024, lineCount)
	fmt.Println()

	// Test GoTTP Parsing with Cable Modem Input (Starlark macros)
	fmt.Println("1. Cable Modem Input Parsing Performance (Starlark Macros):")
	fmt.Println("   Running GoTTP parsing benchmark with Starlark macros...")
	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("GoTTP compilation failed: %v", err)
	}

	inputs := gottp.Inputs{
		"Default_Input": data,
	}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := compiled.Parse(inputs, nil, nil)
		if err != nil {
			t.Fatalf("GoTTP parsing failed: %v", err)
		}
	}
	goParseTimeStarlark := time.Since(start)
	goParseAvgStarlark := goParseTimeStarlark / time.Duration(iterations)
	fmt.Printf("   GoTTP (Starlark):     %v total, %v per operation (avg)\n", goParseTimeStarlark, goParseAvgStarlark)

	// Test GoTTP Parsing with Native Go Macros
	fmt.Println("\n2. Cable Modem Input Parsing Performance (Native Go Macros):")
	fmt.Println("   Running GoTTP parsing benchmark with native Go macros...")
	
	// Create runtime and register native Go macros
	runtime := compiled.NewRuntime()
	
	// Register ds_bonded macro
	runtime.GetMacroRegistry().RegisterGoMacro("ds_bonded", func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
		dsIntf, ok := data["ds-intf"].(string)
		if !ok {
			return data, true, nil
		}

		if len(dsIntf) > 0 {
			lastChar := dsIntf[len(dsIntf)-1]
			if lastChar == '*' {
				data["ds-intf"] = dsIntf[:len(dsIntf)-1]
				data["ds-bonded"] = true
				data["ds-impaired"] = false
			} else if lastChar == '#' {
				data["ds-bonded"] = true
				data["ds-impaired"] = true
			} else {
				data["ds-bonded"] = false
				data["ds-impaired"] = false
			}
		}
		return data, true, nil
	})

	// Register us_bonded macro
	runtime.GetMacroRegistry().RegisterGoMacro("us_bonded", func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
		usIntf, ok := data["us-intf"].(string)
		if !ok {
			return data, true, nil
		}

		if len(usIntf) > 0 {
			lastChar := usIntf[len(usIntf)-1]
			if lastChar == '*' {
				data["us-intf"] = usIntf[:len(usIntf)-1]
				data["us-bonded"] = true
				data["us-impaired"] = false
			} else if lastChar == '#' {
				data["us-bonded"] = true
				data["us-impaired"] = true
			} else {
				data["us-bonded"] = false
				data["us-impaired"] = false
			}
		}
		return data, true, nil
	})

	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, err := runtime.Parse(inputs, nil, nil)
		if err != nil {
			t.Fatalf("GoTTP parsing with Go macros failed: %v", err)
		}
	}
	goParseTimeGo := time.Since(start)
	goParseAvgGo := goParseTimeGo / time.Duration(iterations)
	fmt.Printf("   GoTTP (Native Go):    %v total, %v per operation (avg)\n", goParseTimeGo, goParseAvgGo)
	
	goMacroSpeedup := float64(goParseTimeStarlark) / float64(goParseTimeGo)
	fmt.Printf("   Native Go macros:     %.2fx faster than Starlark\n", goMacroSpeedup)

	// Test Python TTP Parsing with Cable Modem Input
	fmt.Println("\n3. Python TTP Parsing Performance:")
	fmt.Println("   Running Python TTP parsing benchmark...")

	// Escape data for Python string
	escapedData := strings.ReplaceAll(data, `\`, `\\`)
	escapedData = strings.ReplaceAll(escapedData, `"""`, `\"\"\"`)

	script := fmt.Sprintf(`
import sys
import time
sys.path.insert(0, '%s/ttp-original')
from ttp import ttp

template = """%s"""
data = """%s"""

iterations = %d
start = time.perf_counter()
for i in range(iterations):
    parser = ttp(template=template)
    parser.add_input(data)
    parser.parse()
end = time.perf_counter()
total = end - start
avg = total / iterations
print(f"{total:.6f}")
print(f"{avg:.9f}")
`, root, template, escapedData, iterations)

	cmd := exec.Command("python3", "-c", script)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Python TTP parsing benchmark failed: %v\nOutput: %s", err, output)
	}

	// Parse Python output
	var pyParseTotal, pyParseAvg float64
	fmt.Sscanf(string(output), "%f\n%f", &pyParseTotal, &pyParseAvg)
	pyParseTime := time.Duration(pyParseTotal * float64(time.Second))
	pyParseAvgTime := time.Duration(pyParseAvg * float64(time.Second))
	fmt.Printf("   Python TTP: %v total, %v per operation (avg)\n", pyParseTime, pyParseAvgTime)

	parseSpeedupStarlark := float64(pyParseTime) / float64(goParseTimeStarlark)
	parseSpeedupGo := float64(pyParseTime) / float64(goParseTimeGo)
	fmt.Printf("   Speedup (Starlark): %.2fx faster than Python TTP\n", parseSpeedupStarlark)
	fmt.Printf("   Speedup (Native Go): %.2fx faster than Python TTP\n\n", parseSpeedupGo)

	// Summary
	fmt.Println("=== Cable Modem Large Input Summary ===")
	fmt.Printf("GoTTP (Starlark):  %.2fx faster than Python TTP\n", parseSpeedupStarlark)
	fmt.Printf("GoTTP (Native Go): %.2fx faster than Python TTP\n", parseSpeedupGo)
	fmt.Printf("Native Go macros:  %.2fx faster than Starlark macros\n", goMacroSpeedup)
	fmt.Printf("Input size:        ~%d KB (%d lines)\n", len(data)/1024, lineCount)
	fmt.Printf("File:              %s\n", dataPath)
	fmt.Println()
}

