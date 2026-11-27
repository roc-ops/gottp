"""Build gottp cases from recorded TTP 0.10.1 tests, run them, compare strictly."""
import json, subprocess, sys, collections, re

recs = [json.loads(l) for l in open("/tmp/ttp-harness/recorded.jsonl")]
INIT_KW = ["data", "template", "log_level", "log_file", "base_path", "vars", "dynamic_path_key_to_int"]
ALLOWED = {"__init__", "add_input", "parse", "result"}

def text(v):
    if isinstance(v, dict) and "text" in v: return v["text"], "file"
    if isinstance(v, str): return v, None
    return None, "nonstr"

def classify(r):
    """returns (category, case_or_None, expected)"""
    if not r["passed"]: return "py-fails", None, None
    calls = r["calls"]
    ids = list(dict.fromkeys(c["id"] for c in calls))
    if not ids: return "no-ttp-object", None, None
    if len(ids) > 1: return "multi-parser", None, None
    meths = {c["m"] for c in calls}
    extra = meths - ALLOWED
    if extra: return "api:" + ",".join(sorted(extra)), None, None
    init = next(c for c in calls if c["m"] == "__init__")
    kw = dict(zip(INIT_KW, init["args"])); kw.update(init["kwargs"])
    if kw.get("base_path") or kw.get("dynamic_path_key_to_int"): return "init-kwargs", None, None
    tmpl, why = text(kw.get("template", ""))
    if tmpl is None: return "template-nonstr", None, None
    inputs = []
    d = kw.get("data", "")
    if d:
        t, why = text(d)
        if t is None: return "data-" + (list(d)[0] if isinstance(d, dict) else "nonstr"), None, None
        inputs.append({"data": t, "name": "Default_Input"})
    for c in calls:
        if c["m"] == "add_input":
            a = c["args"] + [None]*3
            k = {"data": a[0], "input_name": a[1] or "Default_Input", "template_name": a[2]}
            k.update(c["kwargs"])
            if k.get("template_name") not in (None, "_root_template_") or k.get("groups"):
                return "add_input-kwargs", None, None
            t, why = text(k["data"])
            if t is None: return "data-nonstr", None, None
            inputs.append({"data": t, "name": k["input_name"]})
        if c["m"] == "parse" and (c["args"] or c["kwargs"]) and not all(
                v in (True, False) for v in list(c["kwargs"].values()) + c["args"]):
            pass
    names = [i["name"] for i in inputs]
    if len(names) != len(set(names)): return "same-input-name-x2", None, None
    res = [c for c in calls if c["m"] == "result"]
    if not res: return "no-result", None, None
    rc = res[0]
    if rc["args"] or rc["kwargs"]:
        return "result-kwargs:" + ",".join(sorted(rc["kwargs"])), None, None
    exp = rc.get("ret")
    if isinstance(exp, dict) and "__nonjson__" in exp: return "nonjson-result", None, None
    if "__nonjson__" in json.dumps(exp): return "nonjson-result", None, None
    v = kw.get("vars")
    return "auto", {"id": r["test"], "template": tmpl, "inputs": inputs, "vars": v}, exp

def feats(t):
    f = []
    for name, pat in [("macro", r"<macro"), ("lookup", r"<lookup"), ("output", r"<output"),
                      ("extend", r"<extend"), ("nested-template", r"<template[^>]*>[\s\S]*<template"),
                      ("input-tag", r"<input"), ("vars", r"<vars|<variables"), ("doc", r"<doc"),
                      ("headers", r"_headers_"), ("line", r"_line_"), ("joinmatches", r"joinmatches"),
                      ("per_template", r"per_template"), ("containsall/exclude", r"containsall|exclude"),
                      ("chain", r"chain="), ("ttp-getter", r"get(filename|hostname|time|timestamp)")]:
        if re.search(pat, t): f.append(name)
    return f

def first_diff(a, b, path="$"):
    if type(a) != type(b) and not (isinstance(a, (int, float)) and isinstance(b, (int, float)) and not isinstance(a, bool) and not isinstance(b, bool)):
        return path, a, b
    if isinstance(a, dict):
        for k in sorted(set(a) | set(b), key=str):
            if k not in a: return f"{path}.{k}", "<missing>", b[k]
            if k not in b: return f"{path}.{k}", a[k], "<missing>"
            d = first_diff(a[k], b[k], f"{path}.{k}")
            if d: return d
        return None
    if isinstance(a, list):
        for i in range(min(len(a), len(b))):
            d = first_diff(a[i], b[i], f"{path}[{i}]")
            if d: return d
        if len(a) != len(b): return f"{path}.len", len(a), len(b)
        return None
    return None if a == b else (path, a, b)

cats = collections.Counter(); cases = []; exps = {}
catlist = collections.defaultdict(list)
for r in recs:
    c, case, exp = classify(r)
    cats[c.split(":")[0] if not c.startswith("api") else c] += 1
    catlist[c].append(r["test"])
    if case:
        cases.append(case); exps[case["id"]] = exp
p = subprocess.run(["/tmp/gottp-r"], input="\n".join(json.dumps(c) for c in cases),
                   capture_output=True, text=True, cwd="/tmp/ttp-1da/test/pytest")
outs = [json.loads(l) for l in p.stdout.splitlines()]
rows = []
for case, o in zip(cases, outs):
    exp = exps[case["id"]]
    if o.get("error"):
        st, d = "error", (None, None, o["error"])
    else:
        d = first_diff(exp, o.get("result"))
        st = "pass" if d is None else "fail"
    rows.append({"id": case["id"], "status": st, "diff": d, "feats": feats(case["template"])})
json.dump({"cats": cats, "catlist": catlist, "rows": rows, "cases": cases, "exps": exps,
           "outs": outs}, open(sys.argv[1] if len(sys.argv) > 1 else "/tmp/ttp-harness/replay.json", "w"), indent=1, default=str)
print(dict(cats))
print(collections.Counter(r["status"] for r in rows))
