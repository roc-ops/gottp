"""Runs method calls the way Python TTP 0.10.1's match fallback does, for
pymethod's differential tests.

stdin: a JSON list of {"self": <Python literal>, "call": <call text>}.
stdout: a JSON list with, per call, one of
  {"kind": "value", "repr": <repr of the result>}
  {"kind": "attr"}                    AttributeError: TTP keeps the value
  {"kind": "raise", "type": <class>}  any other exception: TTP's parse fails
The call's arguments are read by TTP's own get_attributes. A tuple result is
shown as a list and a dict view as a list (items as [key, value] lists), as
gottp saves them.
"""
import ast
import json
import sys

from ttp.utils.get_attributes import get_attributes


def canon(x):
    if isinstance(x, (tuple, list)):
        return [canon(i) for i in x]
    if isinstance(x, type({}.items())):
        return [[canon(k), canon(v)] for k, v in x]
    if isinstance(x, (type({}.keys()), type({}.values()))):
        return [canon(i) for i in x]
    if isinstance(x, dict):
        return {str(k): canon(v) for k, v in sorted(x.items(), key=lambda kv: str(kv[0]))}
    return x


def run(case):
    data = ast.literal_eval(case["self"])
    item = get_attributes(case["call"])[0]
    try:
        attrib = getattr(data, item["name"])
        result = attrib(*item["args"], **item["kwargs"]) if callable(attrib) else attrib
    except AttributeError:
        return {"kind": "attr"}
    except Exception as e:  # what TTP doesn't catch
        return {"kind": "raise", "type": type(e).__name__, "msg": str(e)}
    return {"kind": "value", "repr": repr(canon(result))}


print(json.dumps([run(c) for c in json.load(sys.stdin)]))
