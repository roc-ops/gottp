// Standalone test for HC field drop bug.
//
// Usage (env vars):
//   GOTTP_TEMPLATE=/path/to/show_iftable_detail.ttp \
//   GOTTP_INPUT=/path/to/show_iftable_detail.txt \
//   GOTTP_IFINDEX=20000001 \
//   go test -run TestHCFieldsDrop -v -timeout 120s ./test/standalone/

package standalone

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/roc-ops/gottp"
)

func TestHCFieldsDrop(t *testing.T) {
	templateFile := os.Getenv("GOTTP_TEMPLATE")
	inputFile := os.Getenv("GOTTP_INPUT")
	ifIndexStr := os.Getenv("GOTTP_IFINDEX")

	if templateFile == "" || inputFile == "" {
		t.Skip("Set GOTTP_TEMPLATE and GOTTP_INPUT env vars to run this test")
	}

	targetIndex := int64(20000001)
	if ifIndexStr != "" {
		if v, err := strconv.ParseInt(ifIndexStr, 10, 64); err == nil {
			targetIndex = v
		}
	}

	tmplBytes, err := os.ReadFile(templateFile)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	inputBytes, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("read input: %v", err)
	}

	t.Logf("Template: %s (%d bytes)", templateFile, len(tmplBytes))
	t.Logf("Input: %s (%d bytes, %d lines)", inputFile, len(inputBytes), countLines(inputBytes))

	compiled, err := gottp.CompileTemplate(string(tmplBytes))
	if err != nil {
		t.Fatalf("CompileTemplate: %v", err)
	}

	result, err := compiled.Parse(gottp.Inputs{"Default_Input": string(inputBytes)}, nil, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Navigate to ifEntry list
	entries := findEntries(t, result)
	t.Logf("Total entries: %d", len(entries))

	hcFields := []string{
		"ifHCInOctets", "ifHCInUcastPkts",
		"ifHCOutOctets", "ifHCOutUcastPkts",
		"ifHCInMulticastPkts", "ifHCInBroadcastPkts",
		"ifHCOutMulticastPkts", "ifHCOutBroadcastPkts",
	}

	found := false
	for _, e := range entries {
		entry := e.(map[string]interface{})
		idx := getInt(entry["ifIndex"])
		if idx != targetIndex {
			continue
		}
		found = true
		t.Logf("ifIndex %d: %d fields", idx, len(entry))

		missing := []string{}
		for _, hc := range hcFields {
			if _, ok := entry[hc]; !ok {
				missing = append(missing, hc)
			}
		}

		if len(missing) > 0 {
			j, _ := json.MarshalIndent(entry, "", "  ")
			t.Errorf("ifIndex %d MISSING %d HC fields: %v\nFull entry:\n%s", idx, len(missing), missing, string(j))
		} else {
			t.Logf("ifIndex %d: all %d HC fields present — PASS", idx, len(hcFields))
		}
		break
	}

	if !found {
		t.Errorf("ifIndex %d not found in %d entries", targetIndex, len(entries))
	}
}

func findEntries(t *testing.T, result interface{}) []interface{} {
	t.Helper()
	if resultList, ok := result.([]interface{}); ok && len(resultList) > 0 {
		return walkMap(resultList[0])
	}
	t.Fatal("unexpected result structure")
	return nil
}

func walkMap(v interface{}) []interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for _, child := range val {
			if list, ok := child.([]interface{}); ok && len(list) > 0 {
				if _, ok := list[0].(map[string]interface{}); ok {
					return list
				}
			}
			if result := walkMap(child); result != nil {
				return result
			}
		}
	}
	return nil
}

func getInt(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case uint64:
		return int64(val)
	case float64:
		return int64(val)
	}
	return -1
}

func countLines(data []byte) int {
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	return n
}
