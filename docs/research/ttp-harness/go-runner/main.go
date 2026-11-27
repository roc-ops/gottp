// gottp replay runner: reads JSON cases on stdin (one per line), writes JSON results.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/roc-ops/gottp"
)

type Input struct {
	Data string `json:"data"`
	Name string `json:"name"`
}
type Case struct {
	ID       string                 `json:"id"`
	Template string                 `json:"template"`
	Inputs   []Input                `json:"inputs"`
	Vars     map[string]interface{} `json:"vars"`
}
type Out struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func run(c Case) (out Out) {
	out.ID = c.ID
	defer func() {
		if r := recover(); r != nil {
			out.Error = fmt.Sprintf("panic: %v", r)
		}
	}()
	p := gottp.NewParser()
	if err := p.AddTemplate(c.Template); err != nil {
		out.Error = "template: " + err.Error()
		return
	}
	for _, in := range c.Inputs {
		p.AddInput(in.Data, in.Name)
	}
	if c.Vars != nil {
		p.AddVars(c.Vars)
	}
	if err := p.Parse(); err != nil {
		out.Error = "parse: " + err.Error()
		return
	}
	out.Result = p.Result()
	return
}

func main() {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64<<20), 64<<20)
	enc := json.NewEncoder(os.Stdout)
	for sc.Scan() {
		var c Case
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			enc.Encode(Out{Error: "bad case: " + err.Error()})
			continue
		}
		o := run(c)
		if err := enc.Encode(o); err != nil {
			enc.Encode(Out{ID: c.ID, Error: "encode: " + err.Error()})
		}
	}
}
