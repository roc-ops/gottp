package test

import (
	"testing"
	"github.com/roc-ops/gottp"
)

func TestKeyFieldsInParseResult(t *testing.T) {
	tmpl := `<template>
<group name="ifEntry*" keys="ifIndex">
ifIndex: {{ ifIndex | to_int }}
ifDescr: {{ ifDescr | ORPHRASE }}
</group>
</template>`

	data := `ifIndex: 1
ifDescr: eth 6/0
ifIndex: 2
ifDescr: eth 7/0`

	compiled, err := gottp.CompileTemplate(tmpl)
	if err != nil { t.Fatal(err) }

	result, err := compiled.ParseWithValidation(
		gottp.Inputs{"Default_Input": data}, nil, nil,
	)
	if err != nil { t.Fatal(err) }

	// Check KeyFields
	keys, ok := result.KeyFields["ifEntry"]
	if !ok {
		t.Fatalf("KeyFields missing 'ifEntry' group, got: %v", result.KeyFields)
	}
	if len(keys) != 1 || keys[0] != "ifIndex" {
		t.Errorf("Expected keys=[ifIndex], got %v", keys)
	}
}

func TestCompoundKeyFields(t *testing.T) {
	tmpl := `<template>
<group name="route*" keys="prefix,vrf">
{{ prefix }} via {{ nexthop }} vrf {{ vrf }}
</group>
</template>`

	data := `10.0.0.0/24 via 192.168.1.1 vrf PROD
10.0.1.0/24 via 192.168.1.2 vrf DEV`

	compiled, err := gottp.CompileTemplate(tmpl)
	if err != nil { t.Fatal(err) }

	result, err := compiled.ParseWithValidation(
		gottp.Inputs{"Default_Input": data}, nil, nil,
	)
	if err != nil { t.Fatal(err) }

	keys, ok := result.KeyFields["route"]
	if !ok {
		t.Fatalf("KeyFields missing 'route' group")
	}
	if len(keys) != 2 || keys[0] != "prefix" || keys[1] != "vrf" {
		t.Errorf("Expected keys=[prefix, vrf], got %v", keys)
	}
}

func TestNoKeysAttribute(t *testing.T) {
	tmpl := `<template>
<group name="entry*">
name: {{ name }}
</group>
</template>`

	data := `name: test`

	compiled, err := gottp.CompileTemplate(tmpl)
	if err != nil { t.Fatal(err) }

	result, err := compiled.ParseWithValidation(
		gottp.Inputs{"Default_Input": data}, nil, nil,
	)
	if err != nil { t.Fatal(err) }

	if len(result.KeyFields) != 0 {
		t.Errorf("Expected empty KeyFields, got %v", result.KeyFields)
	}
}
