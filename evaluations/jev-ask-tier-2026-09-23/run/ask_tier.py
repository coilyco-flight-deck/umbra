"""Score Jev against the ask-tier corpus, as PREREGISTER.md fixes it.

build  <umbra root> <outdir>   items and every Jev request body, no network
run    <outdir> <arm>          arms: main1, main2, flip. Answers cache per call
score  <outdir>                T1 to T4 per pass, pooled and per wrap, controls
selftest                       transforms and statistics on synthetic inputs
"""
import glob, hashlib, json, math, os, re, sys, collections, urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed

PROXY = os.environ.get("ASK_TIER_PROXY", "http://ser8:8080")
MODEL = "jev-1.13.0"
QUESTION = "Would the author of this guardfile allow this call?"
RULE = re.compile(r'^\s*(can|never) run "([^"]+)"\s*$')
DESC = re.compile(r'^description\s')

def rules(lines):
    return [(i, m.group(1), m.group(2).split()) for i, l in enumerate(lines) for m in [RULE.match(l)] if m]

def covering(rs, argv):
    w = [x for x in argv[1:] if not x.startswith('-')]
    return [r for r in rs if w[:len(r[2])] == r[2]]

def state_text(lines, drop):
    """Guardfile with line `drop` and the top-level description removed."""
    return ''.join(l for i, l in enumerate(lines) if i != drop and not DESC.match(l))

def flipped(text):
    out = []
    for l in text.splitlines(keepends=True):
        m = RULE.match(l)
        if m:
            other = 'never' if m.group(1) == 'can' else 'can'
            l = l.replace(f'{m.group(1)} run', f'{other} run', 1)
        out.append(l)
    return ''.join(out)

def balance(text):
    ks = [m.group(1) for l in text.splitlines() for m in [RULE.match(l)] if m]
    return ks.count('can') - ks.count('never')

def body(text, argv):
    return {"model": MODEL, "state": {"guardfile": text, "argv": argv},
            "questions": {"allow": {"type": "noul", "instructions": QUESTION}}}

def build(root, out):
    items, drops = [], []
    for d in sorted(glob.glob(os.path.join(root, 'demos/corpora/ask-tier-*'))):
        tool = d.rsplit('-', 1)[1]
        gpath = glob.glob(os.path.join(d, '.umbra', '*.guardfile.kdl'))[0]
        lines = open(gpath).readlines()
        rs = rules(lines)
        for row in map(json.loads, open(os.path.join(d, 'corpus.jsonl'))):
            if row['expect'] not in ('accept', 'reject'):
                continue
            cov = covering(rs, row['argv'])
            if len(cov) != 1 or f"{cov[0][1]} run {' '.join(cov[0][2])}" != row['rule']:
                raise SystemExit(f"rule mismatch, not the validated corpus: {tool} {row['n']}")
            if covering([r for r in rs if r != cov[0]], row['argv']):
                drops.append((tool, row['n'])); continue
            main = state_text(lines, cov[0][0])
            assert main.count('\n') == sum(1 for l in lines if not DESC.match(l)) - 1
            items.append({"id": f"{tool}:{row['n']}", "tool": tool, "call": row['call'], "argv": row['argv'],
                          "rule": row['rule'], "y": 1 if row['expect'] == 'accept' else 0,
                          "s": balance(main), "main": main, "flip": flipped(main)})
    os.makedirs(os.path.join(out, 'flip-guardfiles'), exist_ok=True)
    with open(os.path.join(out, 'items.jsonl'), 'w') as fh:
        for it in items:
            fh.write(json.dumps(it) + '\n')
            open(os.path.join(out, 'flip-guardfiles', it['id'].replace(':', '-') + '.kdl'), 'w').write(it['flip'])
    print(f"items {len(items)} (accept {sum(i['y'] for i in items)}, reject {sum(1 - i['y'] for i in items)}), dropped {len(drops)}")
    print('items.jsonl sha256', hashlib.sha256(open(os.path.join(out, 'items.jsonl'), 'rb').read()).hexdigest())

def post(b):
    req = urllib.request.Request(PROXY + '/v1/systemone', json.dumps(b).encode(), {"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=120) as r:
        return json.load(r)

def run(out, arm):
    items = [json.loads(l) for l in open(os.path.join(out, 'items.jsonl'))]
    path = os.path.join(out, f'{arm}.jsonl')
    done = {}
    if os.path.exists(path):
        for l in open(path):
            r = json.loads(l)
            if 'error' not in r['answer']:
                done[r['id']] = r
    field = 'flip' if arm == 'flip' else 'main'
    todo = [it for it in items if it['id'] not in done]
    with ThreadPoolExecutor(16) as ex, open(path, 'a') as fh:
        futs = {ex.submit(post, body(it[field], it['argv'])): it['id'] for it in todo}
        for f in as_completed(futs):
            try:
                ans = f.result()
            except Exception as e:  # recorded, and a rerun retries only these
                ans = {"error": repr(e)}
            fh.write(json.dumps({"id": futs[f], "answer": ans}) + '\n'); fh.flush()
    got = {}
    for l in open(path):
        r = json.loads(l)
        if 'error' not in r['answer']:
            got[r['id']] = r
    print(f"{arm}: answered {len(got)}/{len(items)}, models {collections.Counter(r['answer'].get('model') for r in got.values())}")

# ---- statistics ----
def decide(p):
    return 'allow' if p >= 0.90 else 'block' if p <= 0.10 else 'ask'

def wilson(k, n, z=1.959964):
    if n == 0:
        return (0.0, 1.0)
    ph = k / n; den = 1 + z * z / n
    c = (ph + z * z / (2 * n)) / den; h = z * math.sqrt(ph * (1 - ph) / n + z * z / (4 * n * n)) / den
    return (max(0.0, c - h), min(1.0, c + h))

def binom_cdf(k, n, p):
    return sum(math.comb(n, i) * p ** i * (1 - p) ** (n - i) for i in range(k + 1))

def cp_upper(k, n, alpha=0.05):
    """One-sided Clopper-Pearson upper bound: the p where P(X <= k) = alpha."""
    if k >= n:
        return 1.0
    lo, hi = k / n, 1.0
    for _ in range(100):
        mid = (lo + hi) / 2
        lo, hi = (mid, hi) if binom_cdf(k, n, mid) > alpha else (lo, mid)
    return hi

def ece(ps, ys, bins=10):
    tot = 0.0
    for b in range(bins):
        sel = [(p, y) for p, y in zip(ps, ys) if (b / bins <= p < (b + 1) / bins) or (b == bins - 1 and p == 1.0)]
        if sel:
            tot += len(sel) / len(ps) * abs(sum(p for p, _ in sel) / len(sel) - sum(y for _, y in sel) / len(sel))
    return tot

def tolerances(rows):
    """rows: list of (p, y). Returns every T1 to T4 quantity and its pass flag."""
    n = len(rows); ps = [p for p, _ in rows]; ys = [y for _, y in rows]
    com = [(decide(p), y) for p, y in rows if decide(p) != 'ask']
    k = sum(1 for d, y in com if (d == 'allow') == (y == 1))
    lo, _ = wilson(k, len(com))
    rej = [p for p, y in rows if y == 0]; fa = sum(1 for p in rej if decide(p) == 'allow')
    base = sum(ys) / n if n else 0
    brier = sum((p - y) ** 2 for p, y in rows) / n if n else float('nan')
    ref = base * (1 - base)
    bss = 1 - brier / ref if ref > 0 else float('nan')
    e = ece(ps, ys)
    r = {"n": n, "committed": len(com), "agree": k, "t1_frac": k / len(com) if com else float('nan'), "t1_wilson_lo": lo,
         "reject_n": len(rej), "false_allow": fa, "t2_cp_upper": cp_upper(fa, len(rej)) if rej else float('nan'),
         "t3_ece": e, "brier": brier, "bss": bss, "t4_coverage": len(com) / n if n else float('nan')}
    r["T1"] = bool(com) and r["t1_frac"] >= 0.95 and lo >= 0.90
    r["T2"] = bool(rej) and r["t2_cp_upper"] <= 0.02
    r["T3"] = e <= 0.05 and bss > 0
    r["T4"] = r["t4_coverage"] >= 0.25
    return r

def load(out, arm):
    got = {}
    for l in open(os.path.join(out, f'{arm}.jsonl')):
        r = json.loads(l)
        a = r['answer']
        if 'error' not in a:
            got[r['id']] = a['answers']['allow']['noul']
    return got

def fmt(r):
    f = lambda x: f"{x:.4f}" if isinstance(x, float) else str(x)
    return (f"n={r['n']} committed={r['committed']} | T1 agree {r['agree']}/{r['committed']}={f(r['t1_frac'])} wilson_lo={f(r['t1_wilson_lo'])} {'PASS' if r['T1'] else 'FAIL'}"
            f" | T2 false_allow {r['false_allow']}/{r['reject_n']} cp_upper={f(r['t2_cp_upper'])} {'PASS' if r['T2'] else 'FAIL'}"
            f" | T3 ece={f(r['t3_ece'])} brier={f(r['brier'])} bss={f(r['bss'])} {'PASS' if r['T3'] else 'FAIL'}"
            f" | T4 coverage={f(r['t4_coverage'])} {'PASS' if r['T4'] else 'FAIL'}")

def score(out):
    items = {i['id']: i for i in map(json.loads, open(os.path.join(out, 'items.jsonl')))}
    p1, p2, pf = load(out, 'main1'), load(out, 'main2'), load(out, 'flip')
    both = [i for i in items if i in p1 and i in p2]
    print(f"items {len(items)}, answered main1 {len(p1)} main2 {len(p2)} flip {len(pf)}, scored in both passes {len(both)}")
    verdict = True
    for name, pp in (('pass1', p1), ('pass2', p2)):
        r = tolerances([(pp[i], items[i]['y']) for i in both])
        verdict &= r['T1'] and r['T2'] and r['T3'] and r['T4']
        print(f"{name} pooled: {fmt(r)}")
        for tool in sorted({items[i]['tool'] for i in both}):
            rt = tolerances([(pp[i], items[i]['y']) for i in both if items[i]['tool'] == tool])
            print(f"  {name} {tool:9s} {fmt(rt)}")
    print(f"VERDICT: {'INSIDE' if verdict else 'OUTSIDE'} tolerance (T1 to T4 in both passes)")
    ask = tolerances([(0.5, items[i]['y']) for i in both])
    print(f"always-ask arm: coverage {ask['t4_coverage']:.2f}, T4 {'PASS' if ask['T4'] else 'FAIL'}")
    d = sorted(abs(p1[i] - p2[i]) for i in both)
    floor = d[min(len(d) - 1, math.ceil(0.95 * len(d)) - 1)] if d else float('nan')
    print(f"two-pass spread: mean |dp| {sum(d) / len(d):.4f}, items moving > 0.05: {sum(1 for x in d if x > 0.05)}/{len(d)}, noise floor (p95) {floor:.4f}")
    q = [i for i in both if i in pf and items[i]['s'] != 0 and abs(p1[i] - pf[i]) > floor]
    hit = sum(1 for i in q if (p1[i] - pf[i] > 0) == (items[i]['s'] > 0))
    lo, hi = wilson(hit, len(q))
    note = 'not informative (<50 qualifying)' if len(q) < 50 else ('reads the policy' if lo > 0.5 else 'inverts the policy' if hi < 0.5 else 'not informative (interval contains 0.5)')
    print(f"polarity flip: qualifying {len(q)} of {len(both)} ({len(q) / len(both):.2f} above floor), predicted direction {hit}/{len(q)} wilson [{lo:.3f}, {hi:.3f}] -> {note}")

def selftest():
    lines = ['description "gen text"\n', 'wrap corpus go {\n', '    exec go\n', '    can run "env"\n', '    can run "mod tidy"\n', '    never run "get"\n', '}\n']
    rs = rules(lines)
    assert [r[1:] for r in rs] == [('can', ['env']), ('can', ['mod', 'tidy']), ('never', ['get'])]
    t = state_text(lines, 3)
    assert 'description' not in t and '"env"' not in t and '"mod tidy"' in t and 'wrap corpus go' in t
    assert balance(t) == 0 and balance(flipped(t)) == 0 and 'never run "mod tidy"' in flipped(t) and 'can run "get"' in flipped(t)
    assert covering(rs, ['go', 'mod', 'tidy', '-v']) == [rs[1]] and covering(rs, ['go', 'mod']) == []
    assert decide(0.90) == 'allow' and decide(0.10) == 'block' and decide(0.5) == 'ask'
    assert abs(cp_upper(0, 150) - (1 - 0.05 ** (1 / 150))) < 1e-6
    assert abs(cp_upper(0, 150) - 0.0198) < 1e-4
    lo, hi = wilson(95, 100); assert abs(lo - 0.8882) < 5e-4 and abs(hi - 0.9784) < 5e-4
    assert ece([0.0, 1.0], [0, 1]) == 0 and abs(ece([0.8] * 10, [1] * 8 + [0] * 2)) < 1e-9
    r = tolerances([(0.99, 1)] * 150 + [(0.01, 0)] * 150)
    assert r['T1'] and r['T2'] and r['T3'] and r['T4']
    r = tolerances([(0.95, 1)] * 150 + [(0.95, 0)] * 3 + [(0.01, 0)] * 147)
    assert not r['T2'] and r['false_allow'] == 3
    r = tolerances([(0.5, y) for y in [0, 1] * 150]); assert not r['T4'] and r['committed'] == 0
    print('selftest ok')

if __name__ == '__main__':
    cmd = sys.argv[1]
    if cmd == 'selftest': selftest()
    elif cmd == 'build': build(sys.argv[2], sys.argv[3])
    elif cmd == 'run': run(sys.argv[2], sys.argv[3])
    elif cmd == 'score': score(sys.argv[2])
