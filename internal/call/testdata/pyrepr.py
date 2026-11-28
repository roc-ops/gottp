"""Prints Python's repr() of every code point (but the surrogates) whose
repr isn't the character in single quotes, and of the strings given as JSON
on stdin, for TestReprMatchesPython."""
import json
import sys

out = {"chars": {}, "strings": []}
for i in range(0x110000):
    if 0xD800 <= i <= 0xDFFF:
        continue
    c = chr(i)
    r = repr(c)
    if r != "'" + c + "'":
        out["chars"][str(i)] = r
out["strings"] = [repr(s) for s in json.load(sys.stdin)]
json.dump(out, sys.stdout)
