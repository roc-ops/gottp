package comparison

import (
	"strings"
	"testing"
)

// TestMatchFunctionString tests string manipulation match functions
func TestMatchFunctionString(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	tests := []struct {
		name     string
		template string
		data     string
	}{
		{
			name: "upper",
			template: `<template results_method="per_input">
<group name="test">
value {{ value | upper }}
</group>
</template>`,
			data: "value hello world",
		},
		{
			name: "lower",
			template: `<group name="test">
value {{ value | lower }}
</group>`,
			data: "value HELLO WORLD",
		},
		{
			name: "strip",
			template: `<group name="test">
value {{ value | strip }}
</group>`,
			data: "value   hello world   ",
		},
		{
			name: "split",
			template: `<group name="test">
value {{ value | split(",") }}
</group>`,
			data: "value a,b,c,d",
		},
		{
			name: "join",
			template: `<group name="test">
value {{ value | join(",") }}
</group>`,
			data: "value a b c d",
		},
		{
			name: "replace",
			template: `<group name="test">
value {{ value | replace("old", "new") }}
</group>`,
			data: "value old text old",
		},
		{
			name: "replaceall",
			template: `<group name="test">
value {{ value | replaceall("old1", "new1", "old2", "new2") }}
</group>`,
			data: "value old1 text old2",
		},
		{
			name: "resub",
			template: `<group name="test">
value {{ value | resub("\\d+", "NUM") }}
</group>`,
			data: "value abc123def456",
			// Python TTP's resub only replaces the first occurrence
			// Expected: "abcNUMdef456" (not "abcNUMdefNUM")
		},
		{
			name: "resuball",
			template: `<group name="test">
value {{ value | resuball("NUM", "\\d+") }}
</group>`,
			data: "value abc123def456",
		},
		{
			name: "prepend",
			template: `<group name="test">
value {{ value | prepend("prefix_") }}
</group>`,
			data: "value suffix",
		},
		{
			name: "append",
			template: `<group name="test">
value {{ value | append("_suffix") }}
</group>`,
			data: "value prefix",
		},
		{
			name: "truncate",
			template: `<group name="test">
value {{ value | truncate(10) }}
</group>`,
			data: "value this is a very long string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunComparison(t, tt.name, tt.template, tt.data, nil, nil)
		})
	}
}

// TestMatchFunctionTypeConversion tests type conversion match functions
func TestMatchFunctionTypeConversion(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	tests := []struct {
		name     string
		template string
		data     string
	}{
		{
			name: "to_int",
			template: `<group name="test">
value {{ value | to_int }}
</group>`,
			data: "value 123",
		},
		{
			name: "to_float",
			template: `<group name="test">
value {{ value | to_float }}
</group>`,
			data: "value 123.456",
		},
		{
			name: "to_str",
			template: `<group name="test">
value {{ value | to_str }}
</group>`,
			data: "value 123",
		},
		{
			name: "to_list",
			template: `<group name="test">
value {{ value | to_list }}
</group>`,
			data: "value a,b,c",
		},
		{
			name: "to_ip",
			template: `<group name="test">
value {{ value | to_ip }}
</group>`,
			data: "value 192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunComparison(t, tt.name, tt.template, tt.data, nil, nil)
		})
	}
}

// TestMatchFunctionIPMAC tests IP and MAC address functions
func TestMatchFunctionIPMAC(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	tests := []struct {
		name     string
		template string
		data     string
	}{
		{
			name: "IP",
			template: `<group name="test">
ip {{ ip | IP }}
</group>`,
			data: "ip 192.168.1.1",
		},
		{
			name: "mac_eui",
			template: `<group name="test">
mac {{ mac | mac_eui }}
</group>`,
			data: "mac 00:11:22:33:44:55",
		},
		{
			name: "to_cidr",
			template: `<group name="test">
mask {{ mask | to_cidr }}
</group>`,
			data: "mask 255.255.255.0",
		},
		{
			name: "to_net",
			template: `<group name="test">
network {{ network | to_net }}
</group>`,
			data: "network 192.168.1.0/24",
		},
		{
			name: "is_ip",
			template: `<group name="test">
ip {{ ip | is_ip }}
</group>`,
			data: "ip 192.168.1.1",
		},
		{
			name: "cidr_match",
			template: `<group name="test">
ip {{ ip | cidr_match("192.168.0.0/16") }}
</group>`,
			data: "ip 192.168.1.1",
		},
		{
			name: "dns",
			template: `<group name="test">
hostname {{ hostname | dns }}
</group>`,
			data: "hostname localhost",
		},
		{
			name: "rdns",
			template: `<group name="test">
ip {{ ip | rdns }}
</group>`,
			data: "ip 127.0.0.1",
		},
		{
			name: "ip_info",
			template: `<group name="test">
ip {{ ip | ip_info }}
</group>`,
			data: "ip 192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip DNS tests as they may have network dependencies
			if tt.name == "dns" || tt.name == "rdns" {
				skipIfKnownDifference(t, "dns")
				return
			}
			RunComparison(t, tt.name, tt.template, tt.data, nil, nil)
		})
	}
}

// TestMatchFunctionConditions tests condition match functions
func TestMatchFunctionConditions(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	tests := []struct {
		name     string
		template string
		data     string
	}{
		{
			name: "isdigit",
			template: `<group name="test">
value {{ value | isdigit }}
</group>`,
			data: "value 123",
		},
		{
			name: "notdigit",
			template: `<group name="test">
value {{ value | notdigit }}
</group>`,
			data: "value abc",
		},
		{
			name: "greaterthan",
			template: `<group name="test">
value {{ value | greaterthan("10") }}
</group>`,
			data: "value 15",
		},
		{
			name: "lessthan",
			template: `<group name="test">
value {{ value | lessthan("100") }}
</group>`,
			data: "value 50",
		},
		{
			name: "contains",
			template: `<group name="test">
value {{ value | contains("test") }}
</group>`,
			data: "value this is a test",
		},
		{
			name: "startswith",
			template: `<group name="test">
value {{ value | startswith("hello") }}
</group>`,
			data: "value hello world",
		},
		{
			name: "endswith",
			template: `<group name="test">
value {{ value | endswith("world") }}
</group>`,
			data: "value hello world",
		},
		{
			name: "equal",
			template: `<group name="test">
value {{ value | equal("test") }}
</group>`,
			data: "value test",
		},
		{
			name: "notequal",
			template: `<group name="test">
value {{ value | notequal("other") }}
</group>`,
			data: "value test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunComparison(t, tt.name, tt.template, tt.data, nil, nil)
		})
	}
}

// TestMatchFunctionDataManipulation tests data manipulation functions
func TestMatchFunctionDataManipulation(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	tests := []struct {
		name     string
		template string
		data     string
	}{
		{
			name: "count",
			template: `<group name="test">
value {{ value | count }}
</group>`,
			data: "value hello world",
		},
		{
			name: "item",
			template: `<group name="test">
value {{ value | item(1) }}
</group>`,
			data: "value a,b,c,d",
		},
		{
			name: "default",
			template: `<group name="test">
value {{ value | default("default_value") }}
</group>`,
			data: "value ",
		},
		{
			name: "sformat",
			template: `<group name="test">
value {{ value | sformat("prefix_{}") }}
</group>`,
			data: "value test",
		},
		{
			name: "joinmatches",
			template: `<group name="test">
{{ value1 | joinmatches }},{{ value2 | joinmatches }}
</group>`,
			data: "value1 a value1 b value2 c value2 d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunComparison(t, tt.name, tt.template, tt.data, nil, nil)
		})
	}
}

// TestMatchFunctionLookup tests lookup functions
func TestMatchFunctionLookup(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<lookup name="test_table" load="python">
test_table = {
    "key1": "value1",
    "key2": "value2",
    "key3": "value3"
}
</lookup>

<group name="test">
key {{ key | lookup("test_table") }}
</group>`

	tests := []struct {
		name string
		data string
	}{
		{
			name: "lookup",
			data: "key key1",
		},
		{
			name: "rlookup",
			data: "key value1",
		},
		{
			name: "gpvlookup",
			data: "key key*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Modify template for rlookup and gpvlookup
			testTemplate := template
			if tt.name == "rlookup" {
				testTemplate = strings.Replace(template, `lookup("test_table")`, `rlookup("test_table")`, 1)
			} else if tt.name == "gpvlookup" {
				testTemplate = strings.Replace(template, `lookup("test_table")`, `gpvlookup("test_table")`, 1)
			}
			RunComparison(t, tt.name, testTemplate, tt.data, nil, nil)
		})
	}
}

// TestMatchFunctionChain tests chain function
func TestMatchFunctionChain(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
my_functions = "upper|strip|split(',')"
</vars>

<group name="test">
value {{ value | chain("my_functions") }}
</group>`

	data := "value  a,b,c  "

	RunComparison(t, "chain", template, data, nil, nil)
}

// TestMatchFunctionToUnicode tests to_unicode function
func TestMatchFunctionToUnicode(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="test">
value {{ value | to_unicode }}
</group>`

	data := "value test"

	RunComparison(t, "to_unicode", template, data, nil, nil)
}
