package main

import (
	"fmt"
	"log"

	"github.com/roc-ops/gottp"
)

func main() {
	// Python-compatible API (stateful)
	parser := gottp.NewParser()

	template := `
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>
`

	// Add template
	if err := parser.AddTemplate(template); err != nil {
		log.Fatalf("Failed to add template: %v", err)
	}

	// Add input
	data1 := `
interface Loopback0
 ip address 192.168.0.1/24
!
`
	parser.AddInput(data1, "Default_Input")

	// Parse
	if err := parser.Parse(); err != nil {
		log.Fatalf("Failed to parse: %v", err)
	}

	// Get results
	result1 := parser.Result()
	fmt.Printf("Result 1: %+v\n\n", result1)

	// Clear input and add new data
	parser.ClearInput()
	data2 := `
interface Vlan100
 ip address 10.0.0.1/24
!
`
	parser.AddInput(data2, "Default_Input")

	// Parse again
	if err := parser.Parse(); err != nil {
		log.Fatalf("Failed to parse: %v", err)
	}

	result2 := parser.Result()
	fmt.Printf("Result 2: %+v\n", result2)
}

