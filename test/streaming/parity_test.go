package streaming_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/roc-ops/gottp"
)

type parityCase struct {
	name     string
	template string
	input    string
	keyField string // primary key for stable sort
}

var parityCases = []parityCase{
	{
		name: "simple_start",
		template: `<group name="entry*">
mac {{ mac | _start_ }}
ip {{ ip }}
</group>`,
		input:    "mac aa\nip 1.1.1.1\nmac bb\nip 2.2.2.2\n",
		keyField: "mac",
	},
}

func TestParseStream_Parity(t *testing.T) {
	for _, tc := range parityCases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := gottp.CompileTemplate(tc.template)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			inputs := gottp.Inputs{"Default_Input": tc.input}

			parseResult, err := c.Parse(inputs, nil, nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			parseRecords := flattenRecords(parseResult)

			var streamRecords []map[string]interface{}
			err = c.ParseStream(inputs, nil, nil,
				func(m map[string]interface{}, sr [2]int, gp string) error {
					streamRecords = append(streamRecords, m)
					return nil
				})
			if err != nil {
				t.Fatalf("ParseStream: %v", err)
			}

			sortByKey(parseRecords, tc.keyField)
			sortByKey(streamRecords, tc.keyField)

			if !reflect.DeepEqual(parseRecords, streamRecords) {
				t.Errorf("parity mismatch\nParse:       %v\nParseStream: %v",
					parseRecords, streamRecords)
			}
		})
	}
}

func flattenRecords(result interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	walk(result, &out)
	return out
}

func walk(v interface{}, out *[]map[string]interface{}) {
	switch t := v.(type) {
	case []interface{}:
		for _, item := range t {
			walk(item, out)
		}
	case map[string]interface{}:
		// A map is a "record" (leaf data) if none of its values are
		// structural containers — i.e. no value is a map[string]interface{}
		// and no value is a []interface{} that itself contains maps.
		// Lists of scalars (e.g. from macro-expanded fields) are fine inside
		// records. Only container-like values cause us to descend.
		if isLeafRecord(t) && len(t) > 0 {
			*out = append(*out, t)
			return
		}
		for _, val := range t {
			walk(val, out)
		}
	}
}

// isLeafRecord returns true when m contains no structural nesting:
// no value is a map[string]interface{} and no value is a
// []interface{} whose elements include maps.
func isLeafRecord(m map[string]interface{}) bool {
	for _, val := range m {
		switch vt := val.(type) {
		case map[string]interface{}:
			return false
		case []interface{}:
			for _, elem := range vt {
				if _, ok := elem.(map[string]interface{}); ok {
					return false
				}
			}
		}
	}
	return true
}

func sortByKey(records []map[string]interface{}, key string) {
	sort.SliceStable(records, func(i, j int) bool {
		ai, _ := records[i][key].(string)
		bi, _ := records[j][key].(string)
		return ai < bi
	})
}

func TestParseStream_Parity_SmallProd(t *testing.T) {
	root := "/Users/jasonpatterson/DH360_Device_Discovery/data/hardware_platforms/casa-systems/casa-chassis/8.8.3.5_build_b851"
	prodCases := []struct {
		name     string
		template string
		input    string
		keyField string
	}{
		{"verbose_dev_sample", "field-mappings/templates/show_cable_modem_verbose.ttp", "raw/show_cable_modem_verbose.txt", "mac-address"},
		{"phy_dev_sample", "field-mappings/templates/show_cable_modem_phy.ttp", "raw/show_cable_modem_phy.txt", "mac-address"},
		{"fec_dev_sample", "field-mappings/templates/show_cable_modem_fec.ttp", "raw/show_cable_modem_fec.txt", "mac-address"},
	}

	for _, pc := range prodCases {
		t.Run(pc.name, func(t *testing.T) {
			tmplBytes, err := os.ReadFile(filepath.Join(root, pc.template))
			if err != nil {
				t.Skipf("template not available: %v", err)
			}
			inputBytes, err := os.ReadFile(filepath.Join(root, pc.input))
			if err != nil {
				t.Skipf("input not available: %v", err)
			}
			c, err := gottp.CompileTemplate(string(tmplBytes))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			inputs := gottp.Inputs{"Default_Input": string(inputBytes)}

			parseResult, err := c.Parse(inputs, nil, nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			parseRecords := flattenRecords(parseResult)

			var streamRecords []map[string]interface{}
			err = c.ParseStream(inputs, nil, nil,
				func(m map[string]interface{}, sr [2]int, gp string) error {
					streamRecords = append(streamRecords, m)
					return nil
				})
			if err != nil {
				t.Fatalf("ParseStream: %v", err)
			}

			sortByKey(parseRecords, pc.keyField)
			sortByKey(streamRecords, pc.keyField)

			if len(parseRecords) != len(streamRecords) {
				t.Fatalf("record count mismatch: Parse=%d ParseStream=%d", len(parseRecords), len(streamRecords))
			}
			for i := range parseRecords {
				if !reflect.DeepEqual(parseRecords[i], streamRecords[i]) {
					t.Errorf("record %d mismatch:\nParse:       %v\nParseStream: %v",
						i, parseRecords[i], streamRecords[i])
				}
			}
		})
	}
}
