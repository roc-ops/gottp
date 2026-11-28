package compiled

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/parser"
)

// dynPathCacheTemplate names each record's path after `site`, which only some
// records match. TTP resolves a missing `site` from its dynamic-path cache
// first and from <vars> second, and it creates that cache fresh for every
// input it parses (ttp.py _results_class.__init__, one per parserObj.parse
// call). Expected values below are TTP 0.10.1 output.
const dynPathCacheTemplate = `
<vars>
site = "DEFAULT"
</vars>
<group name="{{ site }}">
interface {{ interface | _start_ }}
 site {{ site }}
</group>
`

const (
	dynPathInputA = "interface Gi1\n site S1\n" // caches site=S1
	dynPathInputB = "interface Gi2\n"           // no site line
)

func compileDynPathCacheTemplate(t *testing.T) *compiler.CompiledTemplate {
	t.Helper()
	tmpl, err := parser.ParseTemplate(dynPathCacheTemplate)
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	ct, err := compiler.NewCompiler().CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("CompileTemplate: %v", err)
	}
	return ct
}

func parseJSON(t *testing.T, rt *Runtime, inputs map[string]string) string {
	t.Helper()
	res, err := rt.Parse(inputs, nil, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}

// TestDynPathCacheScope pins where a cached dynamic-path value may be used.
func TestDynPathCacheScope(t *testing.T) {
	ct := compileDynPathCacheTemplate(t)

	t.Run("within one input the cache fills later records", func(t *testing.T) {
		got := parseJSON(t, NewRuntime(ct), map[string]string{"Default_Input": dynPathInputA + dynPathInputB})
		want := `[{"S1":[{"interface":"Gi1"},{"interface":"Gi2"}]}]`
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("does not leak into the next Parse on a reused Runtime", func(t *testing.T) {
		rt := NewRuntime(ct)
		parseJSON(t, rt, map[string]string{"Default_Input": dynPathInputA})
		got := parseJSON(t, rt, map[string]string{"Default_Input": dynPathInputB})
		want := `[{"DEFAULT":{"interface":"Gi2"}}]`
		if got != want {
			t.Errorf("Parse(B) after Parse(A) = %s, want %s", got, want)
		}
	})

	t.Run("does not leak between inputs of one Parse", func(t *testing.T) {
		got := parseJSON(t, NewRuntime(ct), map[string]string{"A": dynPathInputA, "B": dynPathInputB})
		want := `[{"S1":{"interface":"Gi1"}},{"DEFAULT":{"interface":"Gi2"}}]`
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})
}

// TestSharedRuntimeConcurrentParseDynamicPath runs one Runtime's Parse from
// many goroutines on a template with dynamic paths. Before #83 this crashed
// with "fatal error: concurrent map writes" in PathResolver.UpdateCache and
// raced on every per-parse field Parse reset on the shared Runtime; run it
// under -race. Each goroutine must also get exactly its own site back.
func TestSharedRuntimeConcurrentParseDynamicPath(t *testing.T) {
	ct := compileDynPathCacheTemplate(t)
	rt := NewRuntime(ct)

	const n = 16
	var wg sync.WaitGroup
	got := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			in := map[string]string{"Default_Input": fmt.Sprintf("interface Gi%d\n site S%d\n", i, i)}
			for j := 0; j < 20; j++ {
				res, err := rt.Parse(in, nil, nil)
				if err != nil {
					errs[i] = err
					return
				}
				b, _ := json.Marshal(res)
				got[i] = string(b)
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		want := fmt.Sprintf(`[{"S%d":{"interface":"Gi%d"}}]`, i, i)
		if got[i] != want {
			t.Errorf("goroutine %d: got %s, want %s", i, got[i], want)
		}
	}
}
