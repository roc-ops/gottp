"""Dumps Python 3.12's per-character str behavior for TestUnicodeMatchesPython.

stdout: JSON {"maps": {code point: [upper, lower, title, casefold]} for the
characters some mapping changes, "classes": [[first, last, bits], ...]}
where bit i of bits is CLASSES[i] of every character first..last.
"""
import json
import sys

CLASSES = ["isalpha", "isdecimal", "isdigit", "isnumeric", "isspace",
           "isprintable", "islower", "isupper", "isalnum", "isidentifier"]

maps = {}
runs = []
for i in range(0x110000):
    if 0xD800 <= i <= 0xDFFF:
        continue
    c = chr(i)
    m = [c.upper(), c.lower(), c.title(), c.casefold()]
    if any(x != c for x in m):
        maps[i] = m
    bits = 0
    for k, name in enumerate(CLASSES):
        if getattr(c, name)():
            bits |= 1 << k
    if runs and runs[-1][1] == i - 1 and runs[-1][2] == bits:
        runs[-1][1] = i
    else:
        runs.append([i, i, bits])
json.dump({"maps": maps, "classes": runs}, sys.stdout)
