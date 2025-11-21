package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

func TestGroupDefaults(t *testing.T) {
	template := `<vars>
defaults = {
  "ds-bonded": False,
  "ds-impaired": False,
  "us-bonded": False,
  "us-impaired": False,
  "rx-power-fluctuating": False,
  "rx-power-maxed": False
}
</vars>
<group name="show-cable-modem" default="defaults">
{{mac-address | MAC | mac_eui}} {{ipv4-address | IP}}    {{us-intf}}       {{ds-intf}}      {{status}}      {{primary-sid}}  {{rx-power}}   {{timing-offset}}   {{num-cpes}}    {{bpi-enabled}}
</group>`

	data := `1042-CMT-C40G-SMR_CMTS-A0999#show cable modem
MAC Address    IP Address      US             DS           MAC         Prim RxPwr  Timing Num  BPI
                               Intf           Intf         Status      Sid  (dBmv) Offset CPEs Enb
000b.c971.664a 10.36.177.77    5/2.1/0*       0/2/10*      online      593  -0.5   2393   0    no 
000b.c971.664f 10.36.177.82    5/4.1/0*       0/4/3*       online      96   0.0    2354   0    no`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	result, err := compiled.Parse(gottp.Inputs{"Default_Input": data}, nil, nil)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	// Convert to JSON for easier inspection
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(resultJSON))

	// Check that defaults are applied
	// Result can be either a map or an array (depending on results method)
	var resultMap map[string]interface{}
	if resultArray, ok := result.([]interface{}); ok && len(resultArray) > 0 {
		// If result is an array, get first element
		if firstElem, ok := resultArray[0].(map[string]interface{}); ok {
			resultMap = firstElem
		}
	} else if rm, ok := result.(map[string]interface{}); ok {
		resultMap = rm
	}
	
	if resultMap != nil {
		if showCableModem, ok := resultMap["show-cable-modem"].([]interface{}); ok && len(showCableModem) > 0 {
			if firstMatch, ok := showCableModem[0].(map[string]interface{}); ok {
				// Check that default values are present
				if _, exists := firstMatch["ds-bonded"]; !exists {
					t.Error("Expected 'ds-bonded' default value to be present")
				}
				if _, exists := firstMatch["ds-impaired"]; !exists {
					t.Error("Expected 'ds-impaired' default value to be present")
				}
				if _, exists := firstMatch["us-bonded"]; !exists {
					t.Error("Expected 'us-bonded' default value to be present")
				}
				if _, exists := firstMatch["us-impaired"]; !exists {
					t.Error("Expected 'us-impaired' default value to be present")
				}
				if _, exists := firstMatch["rx-power-fluctuating"]; !exists {
					t.Error("Expected 'rx-power-fluctuating' default value to be present")
				}
				if _, exists := firstMatch["rx-power-maxed"]; !exists {
					t.Error("Expected 'rx-power-maxed' default value to be present")
				}

				// Check that the values are false (boolean)
				if dsBonded, exists := firstMatch["ds-bonded"]; exists {
					if dsBonded != false {
						t.Errorf("Expected 'ds-bonded' to be false, got %v (type %T)", dsBonded, dsBonded)
					}
				}
			}
		}
	}
}

