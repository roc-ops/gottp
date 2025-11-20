package comparison

import (
	"testing"
)

// TestMatchPatternsRe tests the re pattern with template variable
func TestMatchPatternsRe(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
GE_INTF = "GigabitEthernet\S+"
</vars>

<group>
Internet  {{ ip | re("IP")}}  {{ age | re("\d+") }}   {{ mac }}  ARPA   {{ interface | re("GE_INTF") }}
</group>`

	data := `Protocol  Address     Age (min)  Hardware Addr   Type   Interface
Internet  10.12.13.1        98   0950.5785.5cd1  ARPA   FastEthernet2.13
Internet  10.12.13.3       131   0150.7685.14d5  ARPA   GigabitEthernet2.13
Internet  10.12.13.4       198   0950.5C8A.5c41  ARPA   GigabitEthernet2.17
`

	vars := map[string]interface{}{
		"GE_INTF": "GigabitEthernet\\S+",
	}

	RunComparison(t, "match_patterns_re", template, data, vars, nil)
}

