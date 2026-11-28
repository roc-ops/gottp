package gottp

import "testing"

// PR #179 review: YANG validation sees a record's path variables, which
// forming its path removes from the record.
const pathVarCPUModule = `module t-cpu {
  namespace "urn:t:cpu"; prefix cpu;
  container show { list cpu { key id; leaf id { type uint8; } leaf load { type uint8; } } }
}`

func TestYANGSeesPathVariablesFlatGroup(t *testing.T) {
	tmpl := `<group name="cpu.{{ id }}" yang="t-cpu:show/cpu">
CPU {{ id | to_int }} load {{ load | to_int }}
</group>`
	valid, errs, warns := parseWithYANG(t, tmpl, "CPU 300 load 5\n", pathVarCPUModule, "cpu.{{ id }}")
	requireYANGError(t, valid, errs, warns, "Field 'id' value is out of range")
	valid, errs, warns = parseWithYANG(t, tmpl, "CPU 3 load 5\n", pathVarCPUModule, "cpu.{{ id }}")
	requireYANGValid(t, valid, errs, warns)
}

func TestYANGSeesPathVariablesGroupWithChildren(t *testing.T) {
	tmpl := `<group name="cpu.{{ id }}" yang="t-cpu:show/cpu">
CPU {{ id | to_int }} load {{ load | to_int }}
<group name="info">
 info {{ text }}
</group>
</group>`
	valid, errs, warns := parseWithYANG(t, tmpl, "CPU 300 load 5\n info x\n", pathVarCPUModule, "cpu.{{ id }}")
	requireYANGError(t, valid, errs, warns, "Field 'id' value is out of range")
}

func TestYANGSeesPathVariablesChildGroup(t *testing.T) {
	tmpl := `<group name="dev">
Device {{ name }}
<group name="cpu.{{ id }}" yang="t-cpu:show/cpu">
 CPU {{ id | to_int }} load {{ load | to_int }}
</group>
</group>`
	valid, errs, warns := parseWithYANG(t, tmpl, "Device r1\n CPU 300 load 5\n", pathVarCPUModule, "cpu.{{ id }}[0]")
	requireYANGError(t, valid, errs, warns, "Field 'id' value is out of range")
}
