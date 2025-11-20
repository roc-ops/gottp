package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/roc-ops/gottp"
)

//go:generate gottp-gen -template=template.txt -var=MyTemplate -format=gob

func main() {
	// The template is embedded at compile time via go generate
	// To generate the code, run: go generate
	
	// For this example, we'll compile at runtime since we don't have go generate setup
	templateText := `
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
</group>
`

	// Compile template
	compiled, err := gottp.CompileTemplate(templateText)
	if err != nil {
		log.Fatalf("Failed to compile template: %v", err)
	}

	// Parse data
	data := `
interface Loopback0
 ip address 192.168.0.1/24
 description Router-id-loopback
!
interface Vlan100
 ip address 10.0.0.1/24
 description Management-VLAN
!
`

	inputs := gottp.Inputs{
		"Default_Input": data,
	}

	result, err := compiled.Parse(inputs, nil, nil)
	if err != nil {
		log.Fatalf("Failed to parse: %v", err)
	}

	// Print results as JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	fmt.Println(string(jsonData))
}

