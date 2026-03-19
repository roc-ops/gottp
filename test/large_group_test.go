package test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"github.com/roc-ops/gottp"
)

// TestLargeGroupWithStartMergesAllFields tests that _start_ bypasses maxGapLines,
// allowing 60+ field records to merge fully, and that Starlark macros execute.
func TestLargeGroupWithStartMergesAllFields(t *testing.T) {
	// Build a template with 63 match lines
	var tmplLines []string
	tmplLines = append(tmplLines, `<template><macro name="test_macro" language="starlark">
def test_macro(data):
    data["macro-executed"] = True
    return data
</macro>
<group name="entry*" macro="test_macro">`)
	tmplLines = append(tmplLines, `MAC Address                                      :{{ mac-address | _start_ }}`)
	for i := 1; i <= 60; i++ {
		tmplLines = append(tmplLines, fmt.Sprintf("Field-%02d                                         :{{ field-%02d }}", i, i))
	}
	tmplLines = append(tmplLines, `Last-Field                                       :{{ last-field }}`)
	tmplLines = append(tmplLines, `sysDescr                                         :{{ sys-descr | re(".+") }}`)
	tmplLines = append(tmplLines, `</group></template>`)
	tmpl := strings.Join(tmplLines, "\n")

	// Build input with 2 records
	var inputLines []string
	inputLines = append(inputLines, "MAC Address                                      :38f8.5edd.5e02")
	for i := 1; i <= 60; i++ {
		inputLines = append(inputLines, fmt.Sprintf("Field-%02d                                         :value%02d", i, i))
	}
	inputLines = append(inputLines, "Last-Field                                       :present")
	inputLines = append(inputLines, "sysDescr                                         :DOCSIS 3.1 Cable Modem")
	inputLines = append(inputLines, "")
	inputLines = append(inputLines, strings.Repeat("*", 80))
	inputLines = append(inputLines, "")
	inputLines = append(inputLines, "MAC Address                                      :6cff.ced0.7ab0")
	for i := 1; i <= 60; i++ {
		inputLines = append(inputLines, fmt.Sprintf("Field-%02d                                         :val%02db", i, i))
	}
	inputLines = append(inputLines, "Last-Field                                       :present-b")
	inputLines = append(inputLines, "sysDescr                                         :Another Modem")
	input := strings.Join(inputLines, "\n")

	compiled, err := gottp.CompileTemplate(tmpl)
	if err != nil { t.Fatalf("Compile: %v", err) }

	result, err := compiled.Parse(gottp.Inputs{"Default_Input": input}, nil, nil)
	if err != nil { t.Fatalf("Parse: %v", err) }

	j, _ := json.MarshalIndent(result, "", "  ")

	resultList, ok := result.([]interface{})
	if !ok || len(resultList) == 0 { t.Fatalf("No results") }

	topMap, ok := resultList[0].(map[string]interface{})
	if !ok { t.Fatalf("Top result not map: %T", resultList[0]) }

	entries, ok := topMap["entry"]
	if !ok { t.Fatalf("No 'entry' key in result") }

	entryList, ok := entries.([]interface{})
	if !ok { t.Fatalf("entry is %T, not list", entries) }

	if len(entryList) != 2 {
		t.Errorf("Expected 2 entries, got %d\nResult: %s", len(entryList), j)
	}

	for i, e := range entryList {
		entry, ok := e.(map[string]interface{})
		if !ok { t.Errorf("entry[%d] is %T", i, e); continue }

		// Issue 1: all 63 fields should be present
		if _, ok := entry["last-field"]; !ok {
			t.Errorf("entry[%d]: 'last-field' missing — record truncated", i)
		}
		if _, ok := entry["sys-descr"]; !ok {
			t.Errorf("entry[%d]: 'sys-descr' missing — record truncated", i)
		}
		if _, ok := entry["field-60"]; !ok {
			t.Errorf("entry[%d]: 'field-60' missing — record truncated", i)
		}

		// Issue 2: macro should have executed
		if _, ok := entry["macro-executed"]; !ok {
			t.Errorf("entry[%d]: 'macro-executed' missing — Starlark macro did not run", i)
		}

		if t.Failed() && i == 0 {
			t.Logf("entry[0] has %d fields. Full result:\n%s", len(entry), j)
		}
	}
}
