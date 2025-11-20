package comparison

import (
	"testing"
)

// TestMatchIndicatorExact tests the _exact_ indicator
func TestMatchIndicatorExact(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	// Example from docs: capturing IPv4 configuration with _exact_
	template := `<group name="vrfs">
vrf {{ vrf }}
 <group name="ipv4_config">
 address-family ipv4 unicast {{ _start_ }}{{ _exact_ }}
  maximum prefix {{ limit }} {{ warning }}
 !{{ _end_ }}
 </group>
</group>`

	data := `vrf VRF-A
 address-family ipv4 unicast
  maximum prefix 1000 80
 !
 address-family ipv6 unicast
  maximum prefix 300 80
 !
`

	RunComparison(t, "match_indicator_exact", template, data, nil, nil)
}

// TestMatchIndicatorStartExample1 tests _start_ indicator (Example-1 from docs)
func TestMatchIndicatorStartExample1(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="cdp_peers">
------------------------- {{ _start_ }}
Device ID: {{ peer_hostname }}
Entry address(es):
  IP address: {{ peer_ip }}
</group>`

	data := `switch-a#show cdp neighbors detail
-------------------------
Device ID: switch-b
Entry address(es):
  IP address: 131.0.0.1

-------------------------
Device ID: switch-c
Entry address(es):
  IP address: 131.0.0.2
`

	RunComparison(t, "match_indicator_start_example1", template, data, nil, nil)
}

// TestMatchIndicatorStartExample2 tests _start_ indicator with multiple start patterns (Example-2 from docs)
func TestMatchIndicatorStartExample2(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface Tunnel{{ if_id }}
interface GigabitEthernet{{ if_id | _start_ }}
 description {{ description }}
</group>`

	data := `interface Tunnel2422
 description cpe-1
!
interface GigabitEthernet1/1
 description core-1
`

	RunComparison(t, "match_indicator_start_example2", template, data, nil, nil)
}

// TestMatchIndicatorLine tests the _line_ indicator
func TestMatchIndicatorLine(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group>
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
 ip vrf {{ vrf }}
 {{ port_security_cfg | _line_ | contains("port-security") | joinmatches }}
! {{ _end_ }}
</group>`

	data := `interface Loopback0
 description Router-id-loopback
 ip address 192.168.0.113/24
!
interface Gi0/37
 description CPE_Acces
 switchport port-security
 switchport port-security maximum 5
 switchport port-security mac-address sticky
!
`

	RunComparison(t, "match_indicator_line", template, data, nil, nil)
}

// TestMatchIndicatorIgnoreExample1 tests ignore indicator (Example-1 from docs)
func TestMatchIndicatorIgnoreExample1(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `{{ interface }} is up, line protocol is up
  Hardware is Gt96k FE, address is {{ ignore }} (bia {{MAC}})
  MTU {{ mtu }} bytes, BW 100000 Kbit/sec, DLY 1000 usec,`

	data := `FastEthernet0/0 is up, line protocol is up
  Hardware is Gt96k FE, address is c201.1d00.0000 (bia c201.1d00.1234)
  MTU 1500 bytes, BW 100000 Kbit/sec, DLY 1000 usec,
FastEthernet0/1 is up, line protocol is up
  Hardware is Gt96k FE, address is b20a.1e00.8777 (bia c201.1d00.1111)
  MTU 1500 bytes, BW 100000 Kbit/sec, DLY 1000 usec,
`

	RunComparison(t, "match_indicator_ignore_example1", template, data, nil, nil)
}

// TestMatchIndicatorIgnoreExample2 tests ignore indicator with template variable pattern
func TestMatchIndicatorIgnoreExample2(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
pattern_var = "c201\\.1d00\\.0000|b20a\\.1e00\\.8777"
</vars>

{{ interface }} is up, line protocol is up
  Hardware is Gt96k FE, address is {{ ignore("pattern_var") }} (bia {{MAC}})
  MTU {{ mtu }} bytes, BW 100000 Kbit/sec, DLY 1000 usec,`

	data := `FastEthernet0/0 is up, line protocol is up
  Hardware is Gt96k FE, address is c201.1d00.0000 (bia c201.1d00.1234)
  MTU 1500 bytes, BW 100000 Kbit/sec, DLY 1000 usec,
FastEthernet0/1 is up, line protocol is up
  Hardware is Gt96k FE, address is b20a.1e00.8777 (bia c201.1d00.1111)
  MTU 1500 bytes, BW 100000 Kbit/sec, DLY 1000 usec,
`

	vars := map[string]interface{}{
		"pattern_var": "c201\\.1d00\\.0000|b20a\\.1e00\\.8777",
	}

	RunComparison(t, "match_indicator_ignore_example2", template, data, vars, nil)
}

