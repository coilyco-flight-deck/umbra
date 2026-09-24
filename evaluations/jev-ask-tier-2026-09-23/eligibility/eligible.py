"""Eligible n for the ask-tier measurement (PREREGISTER.md, leave-one-rule-out).

A row is eligible when removing its deciding rule leaves the call uncovered. The
matcher is validated against the engine first: on the full guardfile it must
reproduce every row's generator-verified `rule`, or no count is printed.
Usage: eligible.py <umbra repo root> | eligible.py --selftest
"""
import glob, json, os, re, sys, collections

RULE = re.compile(r'^\s*(can|never) run "([^"]+)"\s*$')

def rules(path):
    out = []
    for line in open(path):
        m = RULE.match(line)
        if m:
            out.append((m.group(1), m.group(2).split()))
        elif re.match(r'^\s*(can|never|withhold|deny)', line):
            raise SystemExit(f"unsupported rule shape, matcher not valid here: {path}: {line.strip()}")
    return out

def words(argv):
    # Positionals only. No guardfile here declares a value flag.
    return [w for w in argv[1:] if not w.startswith('-')]

def covering(rs, argv):
    w = words(argv)
    return [(k, p) for k, p in rs if w[:len(p)] == p]

def label(k, p):
    return f"{k} run {' '.join(p)}"

def selftest():
    rs = [('can', ['mod', 'tidy']), ('never', ['mod', 'init']), ('can', ['env'])]
    assert covering(rs, ['go', 'env', 'GOOS']) == [('can', ['env'])]
    assert covering(rs, ['go', 'mod', 'tidy', '-v']) == [('can', ['mod', 'tidy'])]
    assert covering(rs, ['go', '-x', 'mod', 'init']) == [('never', ['mod', 'init'])]
    assert covering(rs, ['go', 'mod']) == []
    assert covering(rs[1:], ['go', 'mod', 'tidy']) == []
    print('selftest ok')

if sys.argv[1] == '--selftest':
    selftest(); sys.exit()
root = sys.argv[1]
tally = collections.Counter(); drops = collections.defaultdict(list); bad = []
per = collections.defaultdict(collections.Counter)
for d in sorted(glob.glob(os.path.join(root, 'demos/corpora/ask-tier-*'))):
    tool = d.rsplit('-', 1)[1]
    rows = [json.loads(l) for l in open(os.path.join(d, 'corpus.jsonl'))]
    rs = rules(glob.glob(os.path.join(d, '.umbra', '*.guardfile.kdl'))[0])
    for r in rows:
        if r['expect'] not in ('accept', 'reject'):
            continue
        cls = r['expect']
        cov = covering(rs, r['argv'])
        # The full guardfile must yield exactly the engine's recorded rule.
        if len(cov) != 1 or label(*cov[0]) != r['rule']:
            bad.append((tool, r['n'], r['call'], r['rule'], [label(*c) for c in cov]))
            continue
        rest = [x for x in rs if x != cov[0]]
        after = covering(rest, r['argv'])
        if after:
            drops[cls].append((tool, r['call'], [label(*c) for c in after]))
            per[tool][cls + '_dropped'] += 1
        else:
            tally[cls] += 1
            per[tool][cls] += 1
print(f"matcher disagreements with the engine's recorded rule: {len(bad)}")
for b in bad:
    print('  MISMATCH', *b)
if bad:
    raise SystemExit('matcher not validated, no eligible count reported')
for cls in ('accept', 'reject'):
    print(f"{cls}: eligible {tally[cls]}  dropped {len(drops[cls])}  floor 150  {'MEETS' if tally[cls] >= 150 else 'SHORT'}")
    for t, call, by in drops[cls]:
        print(f"  dropped {t} {call!r} caught after removal by {by}")
for t in sorted(per):
    print(f"  {t}: accept {per[t]['accept']} reject {per[t]['reject']} dropped {per[t]['accept_dropped'] + per[t]['reject_dropped']}")
