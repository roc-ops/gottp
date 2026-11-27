import json,sys
d=json.load(open("/tmp/ttp-harness/replay.json"))
outs={o["id"]:o for o in d["outs"]}
for name in sys.argv[1:]:
    i=[c for c in d["cases"] if c["id"].endswith("::"+name)][0]
    print("=====",i["id"]); print(i["template"][:1500]); print("--inputs:", json.dumps(i["inputs"])[:600])
    print("--PY:", json.dumps(d["exps"][i["id"]])[:900]); print("--GO:", json.dumps(outs[i["id"]])[:900])
