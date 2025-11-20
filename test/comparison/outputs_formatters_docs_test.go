package comparison

import (
	"testing"
)

// TestOutputFormatterTableExample1 tests table formatter (Example-1 from docs)
func TestOutputFormatterTableExample1(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group>
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>

<output format="table"/>`

	data := `interface Loopback0
 ip address 192.168.0.113/24
!
interface Vlan778
 ip address 2002::fd37/124
!
interface Loopback10
 ip address 192.168.0.10/24
!
interface Vlan710
 ip address 2002::fd10/124
!
`

	RunComparison(t, "output_formatter_table_example1", template, data, nil, nil)
}

// TestOutputFormatterTableExample2 tests table formatter with key and headers (Example-2 from docs)
func TestOutputFormatterTableExample2(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces**.{{ interface }}">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
 ip vrf {{ vrf }}
</group>

<output
path="interfaces"
format="table"
headers="intf, ip, mask, vrf, description, switchport"
key="intf"
missing="Undefined"
/>`

	data := `interface Loopback0
 description Router-id-loopback
 ip address 192.168.0.113/24
!
interface Loopback1
 description Router-id-loopback
 ip address 192.168.0.1/24
!
interface Vlan778
 ip address 2002::fd37/124
 ip vrf CPE1
!
interface Vlan779
 ip address 2002::bbcd/124
 ip vrf CPE2
!
`

	RunComparison(t, "output_formatter_table_example2", template, data, nil, nil)
}

// TestOutputFormatterCSVExample tests CSV formatter from docs
func TestOutputFormatterCSVExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group>
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>

<output format="csv" returner="terminal"/>`

	data := `interface Loopback0
 ip address 192.168.0.113/24
!
interface Vlan778
 ip address 2002::fd37/124
!
`

	RunComparison(t, "output_formatter_csv_example", template, data, nil, nil)
}

