// Tests for editor/js/result-path.js. Run from the repo root:
//   node --test editor/test/*.test.js
'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const rp = require('../js/result-path.js');

// gottp's output and ResultPaths for keys with "." (#76, PR #179): a top
// "_" group (merged into the root), interfaces.{{ interface }} with a child
// ips*, and bgp.{{ peer }}.
const data = [{
    bgp: { '1.1.1.1': { asn: '65000' } },
    hostname: 'r1',
    interfaces: {
        'Gi0/0.10': { desc: 'up', ips: [{ ip: '1.1.1.1' }] },
        Gi1: { desc: 'down' },
    },
}];
const text = JSON.stringify(data, null, 2);
const lines = text.split('\n');
const lineOf = (needle, from = 0) => lines.findIndex((l, i) => i >= from && l.includes(needle)) + 1;
const colOf = (line, needle) => lines[line - 1].indexOf(needle) + 1;

test('splitResultPath unescapes and splits on unescaped dots only', () => {
    assert.deepEqual(rp.splitResultPath('interfaces.Gi0/0\\.10'), ['interfaces', 'Gi0/0.10']);
    assert.deepEqual(rp.splitResultPath('bgp.1\\.1\\.1\\.1'), ['bgp', '1.1.1.1']);
    assert.deepEqual(rp.splitResultPath('interfaces.Gi0/0\\.10.ips[0]'), ['interfaces', 'Gi0/0.10', 'ips[0]']);
    assert.deepEqual(rp.splitResultPath('saps.lag-4:1629\\.\\*'), ['saps', 'lag-4:1629.*']);
    assert.deepEqual(rp.splitResultPath('a\\\\.b'), ['a\\', 'b']);
    assert.deepEqual(rp.splitResultPath(''), []);
});

test('splitResultPath drops unescaped trailing formatters only', () => {
    assert.deepEqual(rp.splitResultPath('vrfs**.rts*'), ['vrfs', 'rts']);
    assert.deepEqual(rp.splitResultPath('x.a\\*'), ['x', 'a*']);
    assert.deepEqual(rp.splitResultPath('x.a\\**'), ['x', 'a*']);
});

test('joinResultPath escapes as gottp does, and round-trips', () => {
    assert.equal(rp.joinResultPath(['interfaces', 'Gi0/0.10']), 'interfaces.Gi0/0\\.10');
    assert.equal(rp.joinResultPath(['x', { key: '1.1.1.1', index: 2 }]), 'x.1\\.1\\.1\\.1[2]');
    for (const p of ['interfaces.Gi0/0\\.10.ips[0]', 'bgp.1\\.1\\.1\\.1', 'a\\\\.b\\*', '']) {
        assert.equal(rp.normalizeResultPath(p), p);
    }
    assert.equal(rp.childResultPath('', 'hostname'), 'hostname');
    assert.equal(rp.childResultPath('bgp.1\\.1\\.1\\.1', 'asn'), 'bgp.1\\.1\\.1\\.1.asn');
});

test('output to input: the key under the cursor gives the escaped ResultPath', () => {
    const gi = lineOf('"Gi0/0.10"');
    assert.equal(rp.resultPathAt(text, gi, colOf(gi, 'Gi0/0.10')), 'interfaces.Gi0/0\\.10');
    const desc = lineOf('"desc"', gi);
    assert.equal(rp.resultPathAt(text, desc, colOf(desc, 'desc')), 'interfaces.Gi0/0\\.10.desc');
    const ip = lineOf('"ip"');
    assert.equal(rp.resultPathAt(text, ip, colOf(ip, '"ip"') + 1), 'interfaces.Gi0/0\\.10.ips[0].ip');
    const peer = lineOf('"1.1.1.1"');
    assert.equal(rp.resultPathAt(text, peer, colOf(peer, '1.1.1.1')), 'bgp.1\\.1\\.1\\.1');
    const asn = lineOf('"asn"');
    assert.equal(rp.resultPathAt(text, asn, colOf(asn, 'asn')), 'bgp.1\\.1\\.1\\.1.asn');
    // A top "_" record's keys are at the root.
    const host = lineOf('"hostname"');
    assert.equal(rp.resultPathAt(text, host, colOf(host, 'hostname')), 'hostname');
    // Not on a key.
    assert.equal(rp.resultPathAt(text, host, lines[host - 1].length), null);
});

test('input to output: an escaped ResultPath finds its key', () => {
    const at = (p, order) => {
        const e = rp.locateResultPath(text, p, order);
        return e && [e.line, e.col];
    };
    const gi = lineOf('"Gi0/0.10"');
    assert.deepEqual(at('interfaces.Gi0/0\\.10'), [gi, colOf(gi, '"Gi0/0.10"')]);
    const desc = lineOf('"desc"', gi);
    assert.deepEqual(at('interfaces.Gi0/0\\.10.desc'), [desc, colOf(desc, '"desc"')]);
    const item = lineOf('{', lineOf('"ips"'));
    assert.deepEqual(at('interfaces.Gi0/0\\.10.ips[0]'), [item, colOf(item, '{')]);
    // The array key with a match order goes to that item.
    assert.deepEqual(at('interfaces.Gi0/0\\.10.ips', 0), [item, colOf(item, '{')]);
    const peer = lineOf('"1.1.1.1"');
    assert.deepEqual(at('bgp.1\\.1\\.1\\.1'), [peer, colOf(peer, '"1.1.1.1"')]);
    // The unescaped form is a different path: Gi0/0 -> 10, which isn't there.
    assert.equal(at('interfaces.Gi0/0.10'), null);
    assert.equal(at('bgp.1.1.1.1'), null);
    // "" is the root; its variables are root keys.
    assert.deepEqual(at(''), [1, 1]);
    const host = lineOf('"hostname"');
    assert.deepEqual(at(rp.childResultPath('', 'hostname')), [host, colOf(host, '"hostname"')]);
});

test('every key of the output round-trips through its ResultPath', () => {
    for (const e of rp.scanJSONKeys(text)) {
        const found = rp.locateResultPath(text, e.path);
        assert.ok(found, e.path);
        if (e.kind === 'key') {
            assert.equal(rp.resultPathAt(text, e.line, e.col + 1), e.path);
        }
    }
});

test('keys with quotes, backslashes and stars', () => {
    const t = JSON.stringify([{ 'a\\b': { 'x"y': 1, 'lag-4:1629.*': {} } }], null, 2);
    const paths = rp.scanJSONKeys(t).map(e => e.path);
    assert.deepEqual(paths, ['a\\\\b', 'a\\\\b.x"y', 'a\\\\b.lag-4:1629\\.\\*']);
    assert.ok(rp.locateResultPath(t, 'a\\\\b.lag-4:1629\\.\\*'));
});
