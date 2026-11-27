# gottp's YANG validator: what it enforces, and the smallest useful schema

Research for roc-ops/gottp#38 (parent map #37). Facts only. Line numbers refer to commit `27b1e4f`.
Every behavior marked **(probed)** was confirmed by running the public `gottp` API from a scratch
module in `/tmp/yangprobe` (it uses a `replace` directive pointing at this checkout).

## TL;DR

- A group opts in with the **`yang="<module-or-prefix>:<path/to/node>"`** attribute. The path must name the
  YANG node that **each match** maps to. For a `cpu*` list group that is the `list` node, not its container.
  `functions="validate(...)"` is a separate, Cerberus-style checker. It is not YANG.
- What is actually enforced: `mandatory` (only on direct children that are present), base-type kind,
  `range` (explicit **and** the implicit range of each int/uint type), `length`, union (by kind), and
  leaf-list elements. Typedefs, groupings/`uses` and imports work because goyang resolves them.
- What is silently ignored: `pattern` (an explicit TODO), enum membership, identityref base, leafref target,
  list keys and their uniqueness, `unique`, `min-elements`/`max-elements`, `default`, `when`, `must`, and
  `choice`/`case`. Leaves inside a case come back as "not defined" warnings.
- A failure never changes or filters the parsed data. It appears only in `ParseResult.ValidationResults[groupName]`.
  `Parse()` throws the results away. The editor shows them as Monaco markers plus a notification.
- Biggest footguns: (1) a `yang=` path one level too high gives **Valid=true** with only warnings;
  (2) int types reject numeric strings, so `to_int` is required, while decimal64 accepts strings;
  (3) a float such as 12.5 passes as `uint8`; (4) a module that fails to parse, or a missing import,
  makes the **whole parse** fail. It is not reported as a validation result.

## 1. Wiring: how a template binds output to a schema

### Loading modules
- Public API: `ParseOptions.YANGModules *YANGModules` (`gottp.go:463-464`). Its fields are `Modules map[name]content`,
  `Files []string` and `URLs []string` (`gottp.go:426-431`).
- `loadYANGModules` (`gottp.go:354-423`) parses every module first and then calls `ProcessAllModules()` once,
  so import order does not matter. Any parse or process error is returned. `ParseWithValidation` then fails
  with `failed to load YANG modules: …` (`gottp.go:305-312`). **(probed)** Unparseable source gives a hard
  parse error. A missing import gives `missing required module: my-types; … please add …` (`module.go:63-99`).
- The map key in `Modules` is only a label passed to goyang (`module.go:43`). Path resolution uses the
  module name and prefix declared *inside* the YANG source (`module.go:103-114`). **(probed)** With the
  label `"m"`, the path `casa-cpuinfo:…` still resolved.

### Binding a group
- The runtime reads `group.Attributes["yang"]`. There are three call sites:
  - `parseGroup` end, top-level and nested: `internal/compiled/runtime.go:4069-4076`. This validates
    `mergedMatches` (`[]map`), so **each match is validated against the node the path names**.
  - The top-level loop of `ParseWithSourceMap` validates the same group again and overwrites the same key
    (`runtime.go:1317-1322`).
  - A nested group that has its own `yang=` attribute (`runtime.go:3280-3288`) is stored under the key `"<name>[<i>]"`.
- Path grammar (`module.go:166-307`): `module:path` or `prefix:path`. Separators may be `/` or `.`. A leading
  `/`, a trailing `*` and per-segment `pfx:` prefixes are stripped. A path with no `:` is searched in every
  module. **(probed)** These all resolved: `casa-cpuinfo:show_cpuinfo/cpu`, `cpu:show_cpuinfo/cpu`,
  `casa-cpuinfo:show_cpuinfo.cpu`, `show_cpuinfo/cpu` and `casa-cpuinfo:/show_cpuinfo/cpu`.
- A bad path is an *error result*, not a parse error. The message lists the available children
  (`validator.go:35-44`, `module.go:241-242`). **(probed)**
- Real-world example: `test/memory/testdata/show_cable-modem.ttp:89`,
  `yang="data-harbor-sw-8.8.3.6-build-c4e8-a01:casa-ios-cli/show_cable-modem/modem-entry"`.
- Groups with no `yang=` attribute produce no entry at all, even when modules are loaded. **(probed)**
- `docs/USER_GUIDE.md` does not mention YANG anywhere (grep finds nothing). The attribute is undocumented.

### Not YANG: `validate()` / `cerberus()`
`internal/functions/group/validate.go` is a Cerberus-style checker, registered as `validate` and `cerberus`
(`functions.go:57-58`). `functions="validate('s')"` looks up a dict schema named `s` in vars or kwargs
(`validate.go:25-44`). It supports `required`, `type`, `min`, `max`, `regex`, `allowed`, `forbidden` and
nested `schema` (`validate.go:67-185`). It writes `_validation_valid_` and `_validation_errors_` **into the
match data** and never filters it (`validate.go:54-63`). If the schema is missing it silently passes
(`validate.go:46-49`). **(probed)** `Vars{"s": {"usage-5s": {"type":"float","max":100}}}` flagged 140
in-band. `internal/validator/` validates template structure only and has no YANG code (`template.go:37-261`).

## 2. Construct-by-construct

What happens per match (`validator.go:101-116`): required fields, then types (recursing into present
containers and lists), then constraints.

| Construct | Status | Evidence |
|---|---|---|
| `mandatory true` | **Checked**, but only for direct children of the node being validated, and the key must be present. A mandatory leaf inside an *absent* container is not checked. | `validator.go:119-142`; probed (absent `sub/need` not reported) |
| `string` | Checked: Go type must be `string` | `validator.go:234-236` |
| int8/16/32/64 | Checked: Go numeric type. **Strings rejected. Floats accepted** (12.5 passes as an int). | `validator.go:237-242`; probed |
| uint8/16/32/64 | Same as int. Negative values are rejected only through the implicit range check. | `validator.go:243-252`; probed (`-5` → "out of range") |
| implicit type bounds | **Checked** (goyang gives the base types a `Range`). uint8=300 and int8=500 are rejected. | probed |
| `decimal64` | Checked: any numeric, **or a numeric string** | `validator.go:256-269`; probed (`"12.50"` passes) |
| `boolean` | Checked: Go `bool` (a string `"yes"` fails) | `validator.go:253-255`; probed |
| `enumeration` | **Membership ignored.** Any string passes. | `validator.go:270-273`; probed (`"sideways"` passes) |
| `identityref` | Base or derivation ignored. Any string passes. | `validator.go:270-273`; probed |
| `leafref` | Always passes | `validator.go:274-276` |
| `empty` | Value must be `nil` | `validator.go:277-279` |
| `union` | Passes if **any** member *kind* matches. Because enum membership is ignored, `union{uint8; enum auto}` accepts any string. | `validator.go:280-289`; probed (`"zzz"` passes) |
| bits, binary, instance-identifier, … | Permissive (`default: true`) | `validator.go:290-292` |
| `range` | **Checked**, including multi-part ranges and `max`. Numeric strings are parsed. A non-numeric string fails the range check as well as the type check (two errors). | `validator.go:308-340, 389-445`; probed (`1..max` rejects 0) |
| `length` | **Checked** for strings, counting **bytes** (`len`), not characters | `validator.go:343-377, 448-466` |
| `pattern` | **Ignored.** The code has an explicit TODO. | `validator.go:379-384`; probed (`"ABC"` vs `[a-z]+` passes) |
| `leaf-list` | Each element of a `[]interface{}` value is type- and range-checked. A single `joinmatches` value stays a scalar string and fails as one. | `validator.go:166-183, 310-326`; spec `2026-03-16-yang-leaflist-validation-design.md`; probed |
| nested container / list in the data | Recursed into when present. List items must arrive as `[]interface{}`. | `validator.go:201-223`; probed (child `cpu*` under a `slot` container, 140 caught) |
| list `key` / key uniqueness | **Ignored.** The key leaf is treated like any other leaf. | no reference to `Key` in `validator.go`; probed (duplicate key passes) |
| `unique` | Ignored | probed |
| `min-elements` / `max-elements` | Ignored | probed (2 items vs `min 3 / max 1` passes) |
| `default` | Ignored (not applied, not used) | probed |
| `when` / `must` | Ignored | probed |
| `choice` / `case` | **Broken.** Case leaves are not looked up through the choice node, so they produce "not defined" warnings and are never type-checked. | `validator.go:151-161`; probed (`ca` warned) |
| typedef | Works (goyang resolves it to a base kind and range) | probed (`typedef percent` range enforced) |
| grouping / `uses` | Works (expanded by `yang.ToEntry`) | probed (range on a grouping leaf enforced) |
| `import` | Works when all modules are supplied. A missing import is a hard error. | `gottp.go:416-420`; probed |
| field present in the data but not in the schema | **Warning** only. `Valid` stays true. | `validator.go:150-161` |

## 3. What a failure looks like

`ParseWithValidation` returns `ParseResult{Data, ValidationResults map[string]*yang.ValidationResult, …}`
(`gottp.go:141-146, 280-350`). Results are keyed by group name *including* the `*` (`"cpu*"`). Nested
groups with their own `yang=` attribute also get a `"cpu*[0]"` entry. The struct has no JSON tags
(`errors.go:6-28`). **(probed)** The JSON form looks like this:

```json
{ "cpu*": { "Valid": false,
    "Errors": [{ "GroupName": "cpu*", "Path": "casa-cpuinfo:show_cpuinfo/cpu",
                 "Message": "Field 'usage-5s' value is out of range in item 1",
                 "Field": "usage-5s", "Line": 0, "Column": 0 }],
    "Warnings": [] } }
```

- `Line` and `Column` are never set (no assignment anywhere in `internal/yang`).
- `Data` is unchanged. Failing items are not dropped or annotated.
- `CompiledTemplate.Parse()` calls `ParseWithValidation` and returns only `.Data` (`gottp.go:205-211`).
  **Validation runs, but its results are discarded.** (probed)
- Warnings are printed through the same `Error()` text: "YANG validation error in group…" (`errors.go:16-21`).

## 4. Editor path

1. YANG modal (`editor/js/app.js:1381-1525`): file, paste or URL → `loadYANGModuleFromContent` → `wasmBridge.loadYANGModule`
   → stored in `state.yangModules[name] = content` (`app.js:1510-1517`).
   **The WASM `loadYANGModule` only wraps the content in JSON and never parses it** (`editor/wasm/main.go:379-407`),
   so garbage still shows "loaded successfully". The error surfaces later as a parse failure.
2. On parse: `yangModulesJSON = JSON.stringify({Modules: state.yangModules})` (`app.js:291-296`) →
   `wasmBridge.parseTemplate(…, yangModulesJSON, …)` 4th arg (`editor/js/wasm-bridge.js:222, 242, 252`) →
   WASM unmarshals into `gottp.YANGModules` (`main.go:100-108`) → `ParseWithValidation` (`main.go:180-191`) →
   returns `validationResults` as a JSON string (`main.go:207-228`).
3. Display: `displayValidationErrors` (`app.js:364, 1585-1622`) counts errors and warnings and calls
   `setYANGValidationErrors` (`editor/js/monaco-config.js:2192-2240`). That function places a marker on the
   `<group … name="X">` line found by regex, so anonymous groups get no marker, and shows a notification.
   It ignores `Valid`.

## 5. Smallest useful schema: Casa `show cpuinfo`-style

There is no Casa `show cpuinfo` sample in this repo. The prod-baseline path it would come from
(`test/comparison/prod_baseline_test.go:33,52`) is not present on this machine. The input below is
therefore a **synthetic** stand-in with the same shape: one row per CPU, with percentage usage columns.

```yang
module casa-cpuinfo {
  namespace "urn:example:casa-cpuinfo";
  prefix cpu;

  typedef percent {
    type decimal64 { fraction-digits 2; range "0..100"; }
  }

  container show_cpuinfo {
    list cpu {
      key "cpu-id";
      leaf cpu-id   { type uint8; mandatory true; }
      leaf usage-5s { type percent; mandatory true; }
      leaf usage-1m { type percent; }
      leaf usage-5m { type percent; }
    }
  }
}
```

```xml
<group name="cpu*" yang="casa-cpuinfo:show_cpuinfo/cpu">
CPU{{ cpu-id | to_int }}  {{ usage-5s | to_float }}%  {{ usage-1m | to_float }}%  {{ usage-5m | to_float }}%
</group>
```

Why each piece matters. Each point was checked with a probe.
- `yang=` must point at **`show_cpuinfo/cpu`** (the list). If it points at `show_cpuinfo`, the result is
  `Valid=true` with 8 "not defined" warnings, and nothing is checked.
- `range "0..100"` on decimal64 is where most of the value lies. It rejects `140.00%` (→ `Field 'usage-5s' value is out of range in item 1`).
- `uint8` gives a free 0..255 bound (CPU300 is rejected). It **requires `to_int`**. Without it every row fails
  with `invalid type string … expected uint8`. decimal64 works with or without `to_float`.
- `mandatory true` on one usage leaf is the only thing that catches a renamed or typo'd capture variable.
  For example `usage_5s` gives "Required field 'usage-5s' is missing", where an optional leaf would give
  only a warning. This is the cheapest guard against template drift.
- `key "cpu-id"` is required for YANG validity of a config list, but it is **not enforced** (duplicate CPU IDs pass).

Results (probed): good input → `Valid=true, 0 errors, 0 warnings`. `CPU1 140.00%` → one error, as shown in §3.
A single-match input is still validated as a list item (`item 0`).

## 6. Validator bugs and gaps found (not fixed)

1. `pattern` is never enforced (`validator.go:379-384`, TODO).
2. Enum and identityref membership is not checked. As a result, `union` with an enum member accepts any string (`validator.go:270-289`).
3. Int and uint kinds accept `float64` with a fractional part (12.5 passes as `uint8`) (`validator.go:237-252`).
4. `choice`/`case` leaves are reported as "not defined" warnings and never type-checked, because lookups use
   `schemaEntry.Dir[name]` directly and do not descend through choice or case entries (`validator.go:151`).
5. `mandatory` is not checked inside an absent container. `min/max-elements`, `unique`, keys, `when`, `must`
   and `default` are all ignored.
6. A `yang=` path at the wrong level passes silently (only warnings, `Valid=true`). Unknown fields are warnings only (`validator.go:150-161`).
7. `CompiledTemplate.Parse()` runs validation and discards it (`gottp.go:205-211`).
8. Top-level groups are validated twice, with the same key overwritten (`runtime.go:1317-1322` and `4069-4076`).
   A nested group with `yang=` is recorded under both `name` and `name[i]` (probed).
9. `ValidationError.Line` and `Column` are never populated. Warnings use the "YANG validation error" text.
10. `length` counts bytes, not characters (`validator.go:449`).
11. Editor: `loadYANGModule` does not parse, so the modal reports success for invalid YANG (`main.go:379-407`, `app.js:1513-1517`).
12. A non-numeric string for decimal64 produces two errors (type, plus a range error from the implicit range) (probed).
13. Undocumented: `docs/USER_GUIDE.md` has no mention of the `yang=` attribute.

## Reproduction

`/tmp/yangprobe` (outside the repo) has a `go.mod` with
`replace github.com/roc-ops/gottp => <this checkout>`. Each probe calls `gottp.CompileTemplate(tmpl)`, then
`ct.ParseWithValidation(gottp.Inputs{"_root_template_": input}, nil, &gottp.ParseOptions{YANGModules: &gottp.YANGModules{Modules: map[string]string{"m": yangSrc}}})`,
and prints `res.Data` and each `res.ValidationResults[k]`. The schema and template are exactly as shown in §5.
The construct probes use one list whose leaves carry `pattern`, `enumeration`, `unique`, `min/max-elements`,
`when`, `must`, `default`, `union`, `uses` and `choice`. `go test ./internal/yang/` passes (7 leaf-list tests,
`validator_test.go:46-167`).
