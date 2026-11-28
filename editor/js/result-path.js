/**
 * ResultPath helpers for source-map navigation.
 *
 * A source map's ResultPath joins result keys with ".". A ".", "*" or "\"
 * inside a key is escaped with "\" (gottp's escapePathKey), so a key such as
 * "Gi0/0.10" or "1.1.1.1" is one segment: "interfaces.Gi0/0\.10". A segment
 * may end in an array index ("smms[0]"), and a GroupName used as a path may
 * end in unescaped "*" formatters ("rts*"). The empty ResultPath is the root
 * of the results (a top group "_" merges its record there).
 *
 * Loaded as a plain script in the editor (globals) and as a CommonJS module
 * by editor/test/result-path.test.js.
 */
(function (root) {
    'use strict';

    /**
     * Split a ResultPath into its segments, unescaped. A segment's trailing
     * unescaped "*"s (path formatters) are dropped; escaped ones are kept.
     * "" (the root) gives [].
     * @param {string} path
     * @returns {string[]}
     */
    function splitResultPath(path) {
        if (!path) {
            return [];
        }
        const segments = [];
        let seg = '';
        // stars counts the trailing unescaped "*"s of seg.
        let stars = 0;
        const end = () => {
            segments.push(stars > 0 ? seg.slice(0, seg.length - stars) : seg);
            seg = '';
            stars = 0;
        };
        for (let i = 0; i < path.length; i++) {
            const c = path[i];
            if (c === '\\' && i + 1 < path.length) {
                seg += path[++i];
                stars = 0;
            } else if (c === '.') {
                end();
            } else {
                seg += c;
                stars = c === '*' ? stars + 1 : 0;
            }
        }
        end();
        return segments;
    }

    /**
     * Escape one result key for a ResultPath, as gottp does.
     * @param {string} key
     * @returns {string}
     */
    function escapeResultPathKey(key) {
        return String(key).replace(/[.*\\]/g, '\\$&');
    }

    /**
     * Join result keys into a ResultPath. A key may be given with an array
     * index as {key, index}, which gives "key[index]".
     * @param {Array<string|{key: string, index: number}>} keys
     * @returns {string}
     */
    function joinResultPath(keys) {
        return keys.map(k => (typeof k === 'object' && k !== null)
            ? escapeResultPathKey(k.key) + '[' + k.index + ']'
            : escapeResultPathKey(k)).join('.');
    }

    /**
     * The canonical form of a ResultPath: formatters dropped, keys escaped.
     * @param {string} path
     * @returns {string}
     */
    function normalizeResultPath(path) {
        return joinResultPath(splitResultPath(path));
    }

    /**
     * Split a segment's array index from its key: "smms[0]" gives
     * {key: "smms", index: 0}; "smms" gives {key: "smms", index: null}.
     * @param {string} segment - an unescaped segment
     */
    function segmentKeyAndIndex(segment) {
        const m = /^(.*)\[(\d+)\]$/s.exec(segment);
        return m ? { key: m[1], index: parseInt(m[2], 10) } : { key: segment, index: null };
    }

    // The last text scanJSONKeys scanned, and its entries.
    let scanCache = { text: null, entries: null };

    /**
     * Scan pretty-printed (or any) JSON text for the position of every
     * object key and of every object in an array, each with its ResultPath.
     * Arrays around the outermost object (a list of inputs' results) are
     * entered without adding to the path. Lines and columns are 1-based;
     * endCol is the column just past the key's closing quote.
     * @param {string} text
     * @returns {Array<{path: string, kind: 'key'|'item', line: number, col: number, endCol: number}>}
     */
    function scanJSONKeys(text) {
        if (scanCache.text === text) {
            return scanCache.entries;
        }
        const entries = [];
        const stack = [];
        let line = 1;
        let col = 1;
        // pathOfValue is the path (escaped segments) of the value starting
        // now in the innermost container.
        const pathOfValue = () => {
            const top = stack[stack.length - 1];
            if (!top) {
                return [];
            }
            if (top.type === 'obj') {
                return top.path.concat([escapeResultPathKey(top.key)]);
            }
            if (top.path.length === 0) {
                return [];
            }
            const p = top.path.slice();
            p[p.length - 1] += '[' + top.idx + ']';
            return p;
        };
        for (let i = 0; i < text.length; i++) {
            const c = text[i];
            if (c === '\n') {
                line++;
                col = 1;
                continue;
            }
            const top = stack[stack.length - 1];
            if (c === '"') {
                let j = i + 1;
                while (j < text.length && text[j] !== '"') {
                    j += text[j] === '\\' ? 2 : 1;
                }
                const token = text.slice(i, j + 1);
                if (top && top.type === 'obj' && top.expectKey) {
                    let key;
                    try {
                        key = JSON.parse(token);
                    } catch (e) {
                        key = token.slice(1, -1);
                    }
                    top.key = key;
                    top.expectKey = false;
                    entries.push({
                        path: top.path.concat([escapeResultPathKey(key)]).join('.'),
                        kind: 'key', line, col, endCol: col + token.length,
                    });
                }
                col += j + 1 - i;
                i = j;
                continue;
            }
            if (c === '{' || c === '[') {
                const path = pathOfValue();
                if (c === '{' && top && top.type === 'arr' && top.path.length > 0) {
                    entries.push({ path: path.join('.'), kind: 'item', line, col, endCol: col + 1 });
                }
                stack.push(c === '{' ? { type: 'obj', path, expectKey: true, key: null } : { type: 'arr', path, idx: 0 });
            } else if (c === '}' || c === ']') {
                stack.pop();
            } else if (c === ',' && top) {
                if (top.type === 'obj') {
                    top.expectKey = true;
                } else {
                    top.idx++;
                }
            }
            col++;
        }
        scanCache = { text, entries };
        return entries;
    }

    /**
     * Where a ResultPath is in JSON text: its key, or the object of an
     * indexed segment ("smms[0]"). "" (the root) is line 1. With matchOrder,
     * a path naming an array goes to that item of it. Returns null when the
     * path isn't in the text.
     * @param {string} text
     * @param {string} path
     * @param {number|null} [matchOrder]
     */
    function locateResultPath(text, path, matchOrder = null) {
        const target = normalizeResultPath(path);
        if (target === '') {
            return { path: '', kind: 'root', line: 1, col: 1, endCol: 1 };
        }
        const entries = scanJSONKeys(text);
        if (matchOrder !== null && matchOrder !== undefined) {
            const item = entries.find(e => e.path === target + '[' + matchOrder + ']');
            if (item) {
                return item;
            }
        }
        return entries.find(e => e.path === target) || null;
    }

    /**
     * The ResultPath of the JSON key at line/column of text, or null.
     * @param {string} text
     * @param {number} line - 1-based
     * @param {number} column - 1-based
     */
    function resultPathAt(text, line, column) {
        const hit = scanJSONKeys(text).find(e => e.kind === 'key' && e.line === line && column >= e.col && column <= e.endCol);
        return hit ? hit.path : null;
    }

    /**
     * The ResultPath of key below the record at path ("" is the root).
     * @param {string} path
     * @param {string} key
     */
    function childResultPath(path, key) {
        return path ? path + '.' + escapeResultPathKey(key) : escapeResultPathKey(key);
    }

    const api = {
        splitResultPath,
        escapeResultPathKey,
        joinResultPath,
        normalizeResultPath,
        segmentKeyAndIndex,
        childResultPath,
        scanJSONKeys,
        locateResultPath,
        resultPathAt,
    };
    if (typeof module !== 'undefined' && module.exports) {
        module.exports = api;
    } else {
        Object.assign(root, api);
    }
})(typeof window !== 'undefined' ? window : this);
