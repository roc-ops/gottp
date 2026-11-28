#!/usr/bin/env python3
"""Regenerate the TTP 0.10.1 suite data in testdata/ from Python TTP.

The Go test (ttpsuite_test.go) only reads testdata/; it never runs Python.
This script is the only thing that should write testdata/cases/*.json.

Two commands:

  record.py expect [--check]
      Re-run every case in testdata/cases/ through the installed Python TTP
      (cwd = testdata/pytest, so relative asset paths resolve) and rewrite each
      case's "expected". With --check, write nothing and exit 1 if any
      expectation would change. Needs the TTP version pinned in
      ../comparison/requirements.txt.

  record.py extract --ttp-src DIR
      Rebuild testdata/ from scratch from a TTP source checkout (the PyPI
      0.10.1 release is commit 1da3a76 of github.com/dmulyalin/ttp; the git
      tag v0.10.1 is a different commit). Runs TTP's test/pytest under the
      ttprec.py recorder, keeps the auto-portable records, writes one JSON
      file per test module and copies the assets the cases read. Needs pytest.
      Run "expect" afterwards to confirm the replay reproduces the recording.

A record is auto-portable when the test uses one ttp object, calls only
__init__/add_input/parse/result, calls result() with no arguments, adds
string inputs with unique names, and gets a JSON-serializable result.

Usage: python -I test/ttpsuite/record.py expect   (from any cwd)
"""
import argparse
import collections
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
TESTDATA = os.path.join(HERE, "testdata")
CASES_DIR = os.path.join(TESTDATA, "cases")
PYTEST_DIR = os.path.join(TESTDATA, "pytest")
REQUIREMENTS = os.path.join(HERE, "..", "comparison", "requirements.txt")

INIT_KW = ["data", "template", "log_level", "log_file", "base_path", "vars",
           "dynamic_path_key_to_int"]
ALLOWED = {"__init__", "add_input", "parse", "result"}
# Asset directories under test/pytest that cases read. The .xlsx file and
# TTP_TEMPLATES_DIR_TEST belong to tests that are not auto-portable.
ASSET_DIRS = ["assets", "mock_data", "yang_modules"]
ASSET_SKIP = {"excel_out_test_excel_formatter_update_source.xlsx"}


def pinned_ttp_version():
    with open(REQUIREMENTS, encoding="utf-8") as f:
        for line in f:
            m = re.match(r"\s*ttp==(\S+)", line)
            if m:
                return m.group(1)
    sys.exit(f"no ttp== pin in {REQUIREMENTS}")


def require_pinned_ttp():
    import ttp  # noqa: F401
    from importlib.metadata import version
    want, got = pinned_ttp_version(), version("ttp")
    if want != got:
        sys.exit(f"installed ttp {got}, but {REQUIREMENTS} pins {want}")
    return got


# ---------------------------------------------------------------- extract

def _text(v):
    if isinstance(v, dict) and "text" in v:
        return v["text"]
    if isinstance(v, str):
        return v
    return None


def classify(r):
    """Return (bucket, case_or_None). Same rules as the research harness."""
    if not r["passed"]:
        return "failing upstream", None
    calls = r["calls"]
    ids = list(dict.fromkeys(c["id"] for c in calls))
    if not ids:
        return "no ttp object", None
    if len(ids) > 1:
        return "several parser objects", None
    extra = {c["m"] for c in calls} - ALLOWED
    if extra:
        return "api: " + ",".join(sorted(extra)), None
    init = next(c for c in calls if c["m"] == "__init__")
    kw = dict(zip(INIT_KW, init["args"]))
    kw.update(init["kwargs"])
    if kw.get("base_path") or kw.get("dynamic_path_key_to_int"):
        return "init kwargs (base_path, dynamic_path_key_to_int)", None
    tmpl = _text(kw.get("template", ""))
    if tmpl is None:
        return "non-string template", None
    inputs = []
    if kw.get("data"):
        t = _text(kw["data"])
        if t is None:
            return "non-string data", None
        # ttp(data=...) adds the input to every template, including child
        # <template> tags; add_input() without template_name adds it to the
        # top template only. Keep the distinction for the Python replay.
        inputs.append({"name": "Default_Input", "data": t, "constructor": True})
    for c in calls:
        if c["m"] != "add_input":
            continue
        a = c["args"] + [None] * 3
        k = {"data": a[0], "input_name": a[1] or "Default_Input", "template_name": a[2]}
        k.update(c["kwargs"])
        if k.get("template_name") not in (None, "_root_template_") or k.get("groups"):
            return "add_input(template_name=/groups=)", None
        t = _text(k["data"])
        if t is None:
            return "non-string data", None
        inputs.append({"name": k["input_name"], "data": t})
    for c in calls:
        if c["m"] == "parse" and (c["args"] or c["kwargs"]):
            return "parse() arguments", None
    names = [i["name"] for i in inputs]
    if len(names) != len(set(names)):
        return "same input name added twice", None
    res = [c for c in calls if c["m"] == "result"]
    if not res:
        return "no result() call", None
    if res[0]["args"] or res[0]["kwargs"]:
        return "result() kwargs", None
    exp = res[0].get("ret")
    if "__nonjson__" in json.dumps(exp):
        return "non-JSON result", None
    if kw.get("vars"):
        return "init vars", None
    module, test = r["test"].split("::", 1)
    module = os.path.splitext(os.path.basename(module))[0]
    case = {"name": f"{module}/{test}", "source": r["test"]}
    if r.get("env"):
        case["env"] = r["env"]  # paths relative to testdata/pytest
    case.update({"template": tmpl, "inputs": inputs, "expected": exp})
    return "auto-portable", case


def cmd_extract(args):
    src = os.path.abspath(args.ttp_src)
    pyt = os.path.join(src, "test", "pytest")
    if not os.path.isdir(pyt):
        sys.exit(f"{pyt} not found")
    require_pinned_ttp()
    with tempfile.TemporaryDirectory() as tmp:
        out = os.path.join(tmp, "recorded.jsonl")
        env = dict(os.environ, TTPREC_OUT=out,
                   PYTHONPATH=HERE + os.pathsep + os.environ.get("PYTHONPATH", ""))
        subprocess.run([sys.executable, "-m", "pytest", "-q", "-p", "ttprec", "-p", "no:cacheprovider"],
                       cwd=pyt, env=env, check=False)
        with open(out, encoding="utf-8") as f:
            recs = [json.loads(line) for line in f]
    buckets = collections.Counter()
    by_module = collections.defaultdict(list)
    for r in recs:
        bucket, case = classify(r)
        buckets[bucket] += 1
        if case:
            by_module[case["name"].split("/")[0]].append(case)
    shutil.rmtree(CASES_DIR, ignore_errors=True)
    os.makedirs(CASES_DIR)
    for module, cases in sorted(by_module.items()):
        _write_cases(os.path.join(CASES_DIR, module + ".json"), cases)
    shutil.rmtree(PYTEST_DIR, ignore_errors=True)
    for d in ASSET_DIRS:
        shutil.copytree(os.path.join(pyt, d), os.path.join(PYTEST_DIR, d),
                        ignore=lambda _dir, names: [n for n in names if n in ASSET_SKIP])
    shutil.copy(os.path.join(src, "LICENSE"), os.path.join(TESTDATA, "LICENSE-TTP"))
    print(f"{len(recs)} records")
    for b, n in buckets.most_common():
        print(f"  {n:4d}  {b}")


# ---------------------------------------------------------------- expect

def _write_cases(path, cases):
    with open(path, "w", encoding="utf-8") as f:
        json.dump(cases, f, indent=1, ensure_ascii=False, sort_keys=False)
        f.write("\n")


def run_ttp(case):
    from ttp import ttp
    saved = dict(os.environ)
    os.environ.update({k: os.path.abspath(v) if os.path.exists(v) else v
                       for k, v in case.get("env", {}).items()})
    try:
        return _run_ttp(ttp, case)
    finally:
        os.environ.clear()
        os.environ.update(saved)


def _run_ttp(ttp, case):
    ctor = [i for i in case["inputs"] if i.get("constructor")]
    parser = ttp(data=ctor[0]["data"] if ctor else "", template=case["template"],
                 log_level="CRITICAL")
    for inp in case["inputs"]:
        if not inp.get("constructor"):
            parser.add_input(inp["data"], input_name=inp["name"])
    parser.parse()
    return json.loads(json.dumps(parser.result()))


def cmd_expect(args):
    ver = require_pinned_ttp()
    os.chdir(PYTEST_DIR)
    changed, total = [], 0
    for fn in sorted(os.listdir(CASES_DIR)):
        if not fn.endswith(".json"):
            continue
        path = os.path.join(CASES_DIR, fn)
        with open(path, encoding="utf-8") as f:
            cases = json.load(f)
        dirty = False
        for case in cases:
            total += 1
            got = run_ttp(case)
            if got != case["expected"]:
                changed.append(case["name"])
                case["expected"] = got
                dirty = True
        if dirty and not args.check:
            _write_cases(path, cases)
    print(f"ttp {ver}: {total} cases, {len(changed)} expectation(s) changed")
    for name in changed:
        print("  changed:", name)
    if args.check and changed:
        sys.exit(1)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    e = sub.add_parser("expect", help="re-record expectations from the installed TTP")
    e.add_argument("--check", action="store_true", help="exit 1 instead of writing changes")
    x = sub.add_parser("extract", help="rebuild testdata/ from a TTP source checkout")
    x.add_argument("--ttp-src", required=True, help="TTP checkout at the 0.10.1 release (1da3a76)")
    args = ap.parse_args()
    {"expect": cmd_expect, "extract": cmd_extract}[args.cmd](args)


if __name__ == "__main__":
    main()
