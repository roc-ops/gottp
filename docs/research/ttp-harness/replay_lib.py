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

