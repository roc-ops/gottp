package gottp

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// Every Parse of a CompiledTemplate shares its group pipelines, except a
// parse whose vars change a default= or chain= the pipeline resolved; that
// parse builds its own, and the shared one is unchanged. Concurrent parses
// share the cache safely (run with -race).
func TestCompiledTemplatePipelineCache(t *testing.T) {
	ct, err := CompileTemplate(`<template>
<vars>
hostname="R1"
</vars>
<group name="intf">
interface {{ name }}
 hn:{{ hn | default("hostname") }}
</group>
</template>`)
	if err != nil {
		t.Fatal(err)
	}
	inputs := Inputs{"Default_Input": "interface Gi1\n"}
	parse := func(vars Vars) string {
		res, err := ct.Parse(inputs, vars, nil)
		if err != nil {
			t.Error(err)
			return ""
		}
		return fmt.Sprint(res)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				if got := parse(nil); !strings.Contains(got, "hn:R1") {
					t.Errorf("no vars: %s", got)
				}
			} else if got := parse(Vars{"hostname": "R2"}); !strings.Contains(got, "hn:R2") {
				t.Errorf("hostname=R2: %s", got)
			}
		}(i)
	}
	wg.Wait()
}
