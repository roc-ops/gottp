package gottp

import (
	"errors"
	"strings"
	"testing"
)

func TestParseStream_NonStreamable_ReturnsError(t *testing.T) {
	tmpl := `<group name="entry*">
desc {{ desc | joinmatches }}
</group>`
	c, err := CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	called := false
	cb := func(m map[string]interface{}, sr [2]int, gp string) error {
		called = true
		return nil
	}
	err = c.ParseStream(Inputs{"Default_Input": "desc foo\ndesc bar\n"}, nil, nil, cb)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrTemplateNotStreamable) {
		t.Errorf("expected errors.Is(err, ErrTemplateNotStreamable); got %v", err)
	}
	if called {
		t.Error("callback should not have been invoked")
	}
}

func TestParseStream_CallbackError_Aborts(t *testing.T) {
	tmpl := `<group name="entry*">
mac {{ mac | _start_ }}
ip {{ ip }}
</group>`
	c, err := CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	input := "mac aa\nip 1.1.1.1\nmac bb\nip 2.2.2.2\nmac cc\nip 3.3.3.3\n"
	count := 0
	sentinel := errors.New("stop now")
	cb := func(m map[string]interface{}, sr [2]int, gp string) error {
		count++
		if count == 2 {
			return sentinel
		}
		return nil
	}
	err = c.ParseStream(Inputs{"Default_Input": input}, nil, nil, cb)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel; got %v", err)
	}
	if count != 2 {
		t.Errorf("expected callback called exactly 2 times; got %d", count)
	}
}

func TestParseStream_EmptyInput_ReturnsNil(t *testing.T) {
	tmpl := `<group name="entry*">
mac {{ mac | _start_ }}
</group>`
	c, _ := CompileTemplate(tmpl)
	called := false
	err := c.ParseStream(Inputs{"Default_Input": ""}, nil, nil,
		func(m map[string]interface{}, sr [2]int, gp string) error {
			called = true
			return nil
		})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if called {
		t.Error("callback should not have been invoked on empty input")
	}
}

func TestParseStream_NilCallback_ReturnsError(t *testing.T) {
	tmpl := `<group name="entry*">
mac {{ mac | _start_ }}
</group>`
	c, _ := CompileTemplate(tmpl)
	err := c.ParseStream(Inputs{"Default_Input": "mac aa\n"}, nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil callback")
	}
}

func TestParseStream_GroupPathStripsTrailingStar(t *testing.T) {
	tmpl := `<group name="my.path.entry*">
mac {{ mac | _start_ }}
ip {{ ip }}
</group>`
	c, _ := CompileTemplate(tmpl)
	var got string
	c.ParseStream(Inputs{"Default_Input": "mac aa\nip 1.1.1.1\n"}, nil, nil,
		func(m map[string]interface{}, sr [2]int, gp string) error {
			got = gp
			return nil
		})
	if !strings.HasSuffix(got, "entry") || strings.HasSuffix(got, "*") {
		t.Errorf("groupPath %q should strip trailing star, got with star or wrong path", got)
	}
}
