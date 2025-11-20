package main

import (
	"fmt"
	"log"

	"github.com/roc-ops/gottp"
	"github.com/roc-ops/gottp/internal/formatters"
)

func main() {
	template := `
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
</group>
`

	data := `
interface Loopback0
 ip address 192.168.0.113/24
 description Router-id-loopback
!
interface Vlan778
 ip address 2002::fd37/124
 description CPE_Acces_Vlan
!
`

	// Compile template once
	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		log.Fatalf("Failed to compile template: %v", err)
	}

	// Use many times with different inputs - no reset needed
	result, err := compiled.Parse(gottp.Inputs{
		"Default_Input": data,
	}, nil, nil)
	if err != nil {
		log.Fatalf("Failed to parse: %v", err)
	}

	// Format as JSON
	jsonFormatter := formatters.NewJSONFormatter()
	jsonStr, err := jsonFormatter.FormatString(result)
	if err != nil {
		log.Fatalf("Failed to format JSON: %v", err)
	}

	fmt.Println("Result (JSON):")
	fmt.Println(jsonStr)
}
