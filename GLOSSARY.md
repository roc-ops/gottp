# gottp

A Go implementation of TTP (Template Text Parser) that turns CLI output from network devices into structured data, with optional YANG validation of the result.

## Language

### Parsing

**Template**:
A TTP template that turns one command's CLI output into structured data.
_Avoid_: parser (on its own), TTP file

**Sample**:
One captured CLI output for a single command on a single software version, used to build or check a template.
_Avoid_: fixture (outside tests), raw file

**Software version**:
The version string a device reports for its operating software; templates and schemas are keyed to it, because the same software version produces the same output shape.
_Avoid_: version (when it could mean a schema revision), firmware, release

**Record**:
One result entry a group produces from the lines between one start pattern match and the next (or an end pattern).
_Avoid_: match (for the whole entry), row

**Start pattern**:
A template line whose match begins a new record of its group: the group's first line, every line of a table group, or a line marked `_start_`.
_Avoid_: start line (for the template side)

**End pattern**:
A template line whose match closes the current record of its group.
_Avoid_: end line (for the template side)

**Unmatched line**:
A non-blank line of a sample that no template pattern matched and that the template did not mark as ignored. Any unmatched line is a failure.
_Avoid_: missed line, dropped row

**Ignored line**:
A line the template deliberately skips, such as a banner, a column header or a separator.
_Avoid_: skipped line

**Catch-all capture**:
A pattern that captures a whole line or arbitrary text without structure, which matches every line and so defeats the unmatched-line check.
_Avoid_: wildcard line

### Validation

**Schema**:
An ad hoc YANG module, generated for one command on one software version, that describes the structured output a template should produce. Optional.
_Avoid_: YANG model (for a generated schema), model (ambiguous with the LLM)

**Standard YANG model**:
A YANG module published by a vendor or standards body, as distinct from a generated schema.

**Revision**:
The revision of a schema file, in the YANG sense.
_Avoid_: version (reserved for software version)

**Self-validating parser**:
A template, optionally paired with a schema, plus its samples and the platform, software version and command it applies to.
_Avoid_: validated template

**Drift**:
Output from a newer sample that fails a self-validating parser built from earlier samples of the same command and software version.
_Avoid_: regression (for device output changes)

**Failure report**:
The structured record a self-validating parser emits when it fails: enough input, template, schema and failing rule to reproduce the failure.

### Generation

**Reference doc**:
Vendor documentation for a command, supplied as optional input to template generation.
_Avoid_: manual, spec
