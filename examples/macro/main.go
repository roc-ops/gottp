package main

import (
	"fmt"
	"log"

	"github.com/roc-ops/gottp"
)

func main() {
	template := `
<macro language="starlark">
def process(data):
    return data.upper()
</macro>
<group name="test">
{{ value | macro("process") }}
</group>
`

	data := `
test value
`

	// Compile template
	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		log.Fatalf("Failed to compile: %v", err)
	}

	// Parse
	result, err := compiled.Parse(gottp.Inputs{
		"Default_Input": data,
	}, nil, nil)
	if err != nil {
		log.Fatalf("Failed to parse: %v", err)
	}

	fmt.Printf("Result: %+v\n", result)
}

