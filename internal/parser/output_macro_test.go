package parser

import "testing"

// Python TTP runs an output's macro and functions attributes in attribute
// order (#142), so the parser records which comes first.
func TestParseOutput_MacroOrder(t *testing.T) {
	tests := []struct {
		output     string
		wantMacro  string
		wantBefore bool
	}{
		{`<output macro="m"/>`, "m", true},
		{`<output macro="m" functions="macro('x')"/>`, "m", true},
		{`<output functions="macro('x')" macro="m"/>`, "m", false},
		{`<output format="json" functions="macro('x')" macro="m"/>`, "m", false},
		{`<output functions="macro('x')"/>`, "", false},
	}
	for _, tt := range tests {
		tmpl, err := ParseTemplate("<template>\n" + tt.output + "\n</template>")
		if err != nil {
			t.Fatalf("%s: %v", tt.output, err)
		}
		if len(tmpl.Outputs) != 1 {
			t.Fatalf("%s: got %d outputs", tt.output, len(tmpl.Outputs))
		}
		o := tmpl.Outputs[0]
		if o.Macro != tt.wantMacro || o.MacroBeforeFunctions != tt.wantBefore {
			t.Errorf("%s: got Macro=%q MacroBeforeFunctions=%v, want %q %v", tt.output, o.Macro, o.MacroBeforeFunctions, tt.wantMacro, tt.wantBefore)
		}
	}
}
