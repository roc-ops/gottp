"""Records testdata/ttp_functions.json, Python TTP 0.10.1's own function
tables (the input, group, match and output entries of ttp_dict_cache.pickle,
which TTP looks function names up in), for TestTTPFunctionNames.

Usage: python -I gen_ttp_functions.py OUT.json  (a Python with ttp==0.10.1)
"""
import json, os, pickle, sys
import ttp

with open(os.path.join(os.path.dirname(ttp.__file__), "ttp_dict_cache.pickle"), "rb") as f:
    tables = pickle.load(f)
out = {scope: sorted(tables[scope]) for scope in ("match", "group", "input", "output")}
with open(sys.argv[1], "w") as f:
    json.dump(out, f, indent=1)
    f.write("\n")
