# Code Generation Example

This example demonstrates how to use `gottp-gen` to embed compiled templates at compile time.

## Usage

1. Install the code generator:
```bash
go install github.com/roc-ops/gottp/cmd/gottp-gen@latest
```

2. Create a template file (e.g., `template.txt`):
```
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
</group>
```

3. Add a `//go:generate` directive to your Go code:
```go
//go:generate gottp-gen -template=template.txt -var=MyTemplate -format=gob
```

4. Run `go generate`:
```bash
go generate
```

This will create a `template_gen.go` file with the compiled template embedded.

5. Use the embedded template:
```go
result, err := MyTemplate.Parse(inputs, vars, nil)
```

## Benefits

- Templates are compiled at build time, not runtime
- No need to ship template files with your binary
- Faster startup (no compilation step)
- Type-safe access to compiled templates

