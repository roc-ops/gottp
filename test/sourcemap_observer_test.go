package test

import (
	"testing"

	"github.com/roc-ops/gottp"
)

// smPaths parses data with source maps on and returns, per input line, the
// "group=path" of every match on it ("" for an unmatched line).
func smPaths(t *testing.T, template, data string) ([]string, *gottp.SourceMap) {
	t.Helper()
	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	res, err := compiled.ParseWithValidation(gottp.Inputs{"Default_Input": data}, nil, &gottp.ParseOptions{EnableSourceMap: true})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	in := res.SourceMap.Inputs["Default_Input"]
	out := make([]string, len(in.Lines))
	for i, line := range in.Lines {
		if line.Matched != (len(line.Matches) > 0) {
			t.Errorf("line %d: Matched=%v with %d matches", i, line.Matched, len(line.Matches))
		}
		for j, m := range line.Matches {
			if j > 0 {
				out[i] += " "
			}
			out[i] += m.GroupName + "=" + m.ResultPath
		}
	}
	return out, res.SourceMap
}

func wantPaths(t *testing.T, got, want []string) {
	t.Helper()
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, safeAt(got, i), want[i])
		}
	}
}

func safeAt(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "<none>"
}

// Refactor A2 (#60): matches of every nesting level get the path of the
// record they went into, with list indexes at every level.
func TestSourceMapObserver_DeepNesting(t *testing.T) {
	template := `<group name="routers*">
router {{ name }}
 <group name="interfaces*">
 interface {{ ifname }}
  <group name="addresses*">
  ip {{ ip }}
  </group>
 </group>
</group>`
	data := "router r1\n interface a\n  ip 1\n  ip 2\n interface b\n  ip 3\nrouter r2\n interface c\n  ip 4\n"
	got, sm := smPaths(t, template, data)
	wantPaths(t, got, []string{
		"routers*=routers[0]",
		"interfaces*=routers[0].interfaces[0]",
		"addresses*=routers[0].interfaces[0].addresses[0]",
		"addresses*=routers[0].interfaces[0].addresses[1]",
		"interfaces*=routers[0].interfaces[1]",
		"addresses*=routers[0].interfaces[1].addresses[0]",
		"routers*=routers[1]",
		"interfaces*=routers[1].interfaces[0]",
		"addresses*=routers[1].interfaces[0].addresses[0]",
	})
	// Columns: the match spans the line, as parseGroup matched it; the
	// variable spans its value.
	m := sm.Inputs["Default_Input"].Lines[2].Matches[0]
	if m.StartCol != 0 || m.EndCol != 6 {
		t.Errorf("line 2 match columns: got %d-%d, want 0-6", m.StartCol, m.EndCol)
	}
	if v := m.Variables["ip"]; v == nil || v.StartCol != 5 || v.EndCol != 6 {
		t.Errorf("line 2 ip span: got %+v, want 5-6", v)
	}
}

// A line a pattern matches but no record uses (here: a child line before
// any parent record, and a record filtered out by contains) is not a
// source of any result: unused matches are not recorded, and matches of a
// dropped record have no ResultPath.
func TestSourceMapObserver_UnusedAndDroppedMatches(t *testing.T) {
	template := `<group name="interfaces*" contains="ip">
interface {{ name }}
 ip address {{ ip }}
</group>`
	data := " ip address 9.9.9.9\ninterface a\n ip address 1.1.1.1\ninterface b\ninterface c\n ip address 2.2.2.2\n"
	got, _ := smPaths(t, template, data)
	wantPaths(t, got, []string{
		"", // before any record: not used
		"interfaces*=interfaces[0]",
		"interfaces*=interfaces[0]",
		"interfaces*=", // record b has no ip: dropped by contains
		"interfaces*=interfaces[1]",
		"interfaces*=interfaces[1]",
	})
}

// Dynamic paths are resolved per record, and a void child group's matches
// have no ResultPath.
func TestSourceMapObserver_DynamicPathAndVoid(t *testing.T) {
	template := `<group name="vlans.{{ id }}">
vlan {{ id }}
 name {{ name }}
 <group name="hidden" void="">
 state {{ state }}
 </group>
</group>`
	data := "vlan 10\n name users\n state up\nvlan 20\n name voice\n"
	got, _ := smPaths(t, template, data)
	wantPaths(t, got, []string{
		"vlans.{{ id }}=vlans.10",
		"vlans.{{ id }}=vlans.10",
		"hidden=",
		"vlans.{{ id }}=vlans.20",
		"vlans.{{ id }}=vlans.20",
	})
}

// record= only records a variable, as TTP's group record function does: it
// never merges records, so each match keeps its own result entry.
func TestSourceMapObserver_RecordAttribute(t *testing.T) {
	template := `<group name="vrfs*" record="vrf">
vrf {{ vrf }} rt {{ rt }}
</group>`
	data := "vrf a rt 1\nvrf b rt 2\nvrf a rt 3\n"
	got, _ := smPaths(t, template, data)
	wantPaths(t, got, []string{
		"vrfs*=vrfs[0]",
		"vrfs*=vrfs[1]",
		"vrfs*=vrfs[2]",
	})
}
