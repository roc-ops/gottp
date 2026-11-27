import json, collections, sys
sys.path.insert(0, "/tmp/ttp-harness")
from replay_lib import first_diff
d = json.load(open(sys.argv[1] if len(sys.argv)>1 else "/tmp/ttp-harness/replay.json"))
def norm(x):  # same rule as test/comparison normalizeJSON: unwrap [ [..] ] one level, recursively
    if isinstance(x, dict): return {k: norm(v) for k, v in x.items()}
    if isinstance(x, list):
        n = [norm(i) for i in x]
        if len(n) == 1 and isinstance(n[0], list): return n[0]
        return n
    if isinstance(x, float) and x.is_integer(): return int(x)
    return x
outs = {o["id"]: o for o in d["outs"]}
res = collections.Counter(); rows = []
for r in d["rows"]:
    o = outs[r["id"]]; e = d["exps"][r["id"]]
    if o.get("error"): s = "error"; fd = o["error"]
    elif first_diff(e, o["result"]) is None: s = "pass-strict"; fd = None
    else:
        fd = first_diff(norm(e), norm(o["result"]))
        s = "pass-normalized" if fd is None else "fail"
    res[s] += 1; rows.append((r["id"], s, fd, r["feats"]))
print(res)
json.dump(rows, open("/tmp/ttp-harness/analyzed.json", "w"), indent=1, default=str)
for i, s, fd, f in rows:
    if s in ("fail", "error"): print(s, i.split("::")[0].replace("test/pytest/",""), i.split("::")[1], "|", json.dumps(fd, default=str)[:230], f)
