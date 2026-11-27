# TTP test-suite harness (research scratch, saved from /tmp)

The scripts behind §4 of `../ttp-0.10.1-compatibility.md`, copied here on 2026-11-27 so they survive `/tmp` cleanup. They are research scratch code, not maintained.

- `ttprec.py`: a pytest plugin that records each TTP test's top-level calls (constructor, `add_input`, `parse`, `result`) under Python TTP 0.10.1. Its output is `recorded.jsonl` (286 records).
- `analyze.py`: sorts the records into auto-portable (200) and the other buckets. Output: `analyzed.json`.
- `replay.py` / `replay_lib.py`: replay the auto-portable cases through gottp with `go-runner/` (`gottp.NewParser()`), comparing strictly and after the comparison suite's normalization. Output: `replay.json`, `sample_table.md`.
- `go-runner/`: a small Go program with a `replace` pointing at a local gottp checkout. Edit its `go.mod` before use.

`recorded.jsonl` contains templates and data from TTP's own test suite (github.com/dmulyalin/ttp, test/pytest). Check TTP's license terms before copying it into the gottp tree.
