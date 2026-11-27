"""pytest plugin: record every ttp() usage per test, for replay in gottp."""
import json, os, copy, pytest
import ttp as ttp_pkg
from ttp.ttp import ttp as TTP

OUT = os.environ.get("TTPREC_OUT", "/tmp/ttp-harness/recorded.jsonl")
_cur = {"calls": None}

def _maybe_file(v):
    if isinstance(v, str) and len(v) < 400 and "\n" not in v and os.path.isfile(v):
        with open(v, encoding="utf-8") as f:
            return {"file": v, "text": f.read()}
    if isinstance(v, str) and len(v) < 400 and "\n" not in v and os.path.isdir(v):
        return {"dir": v}
    return v

def _jsonable(o):
    try:
        json.dumps(o)
        return o
    except Exception:
        return {"__nonjson__": repr(o)[:2000]}

def _wrap(name):
    orig = getattr(TTP, name)
    def w(self, *a, **kw):
        rec = {"m": name, "id": id(self),
               "args": [_jsonable(_maybe_file(x)) for x in a],
               "kwargs": {k: _jsonable(_maybe_file(v)) for k, v in kw.items()}}
        top = _cur.get("depth", 0) == 0
        if _cur["calls"] is not None and top:
            _cur["calls"].append(rec)
        _cur["depth"] = _cur.get("depth", 0) + 1
        try:
            r = orig(self, *a, **kw)
        finally:
            _cur["depth"] -= 1
        if name == "result":
            rec["ret"] = _jsonable(copy.deepcopy(r)) if not isinstance(r, str) else r
        return r
    setattr(TTP, name, w)

for n in ["__init__", "add_input", "add_template", "add_vars", "add_lookup",
          "add_function", "parse", "result", "set_input", "get_input_load",
          "clear_result", "get_template", "get_variables", "clear_input"]:
    if hasattr(TTP, n):
        _wrap(n)

@pytest.hookimpl(hookwrapper=True)
def pytest_runtest_call(item):
    _cur["calls"] = []
    outcome = yield
    calls, _cur["calls"] = _cur["calls"], None
    with open(OUT, "a") as f:
        f.write(json.dumps({"test": item.nodeid, "passed": outcome.excinfo is None,
                            "calls": calls}) + "\n")
