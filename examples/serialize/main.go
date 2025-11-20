package main

import (
	"fmt"
	"log"

	"github.com/roc-ops/gottp"
)

func main() {
	template := `
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>
`

	// Compile template
	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		log.Fatalf("Failed to compile: %v", err)
	}

	// Save to JSON
	jsonData, err := gottp.SaveCompiledTemplate(compiled, "json")
	if err != nil {
		log.Fatalf("Failed to save: %v", err)
	}

	fmt.Printf("Saved template (%d bytes):\n%s\n\n", len(jsonData), string(jsonData))

	// Load from JSON
	loaded, err := gottp.LoadCompiledTemplate(jsonData, "json")
	if err != nil {
		log.Fatalf("Failed to load: %v", err)
	}

	fmt.Println("Successfully loaded compiled template!")

	// Use the loaded template
	data := `
interface Loopback0
 ip address 192.168.0.1/24
!
`

	result, err := loaded.Parse(gottp.Inputs{
		"Default_Input": data,
	}, nil, nil)
	if err != nil {
		log.Fatalf("Failed to parse: %v", err)
	}

	fmt.Printf("Parse result: %+v\n", result)
}

