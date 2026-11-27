#!/usr/bin/env python3
"""B06 lane report builder: CSV shards -> the envelope report on stdout.

The tool READS only: the lane script redirects stdout into the report and
summary files. The contention sanity gate is a truthfulness check on the
methodology: if mean latency does not rise with the level, the fan-out did
not actually press on the single writer, and the envelope would be a
fiction - the report says SOFT (not verified) instead of hiding behind
numbers.

Usage: report.py --format md|json --csv DIR --env FILE --verify FILE
"""
import argparse
import csv
import glob
import json
import math
import os
import statistics


def pctl(values, q):
    if not values:
        return None
    s = sorted(values)
    idx = max(0, min(len(s) - 1, int(math.ceil(q * len(s))) - 1))
    return s[idx]


def read_rows(csv_dir):
    rows = []
    for path in sorted(glob.glob(os.path.join(csv_dir, "*.csv"))):
        with open(path) as fh:
            for rec in csv.DictReader(fh):
                rec["level"] = int(rec["level"])
                rec["pid"] = int(rec["pid"])
                rec["dur_ns"] = int(rec["dur_ns"])
                rec["start_ns"] = int(rec["start_ns"])
                rec["end_ns"] = int(rec["end_ns"])
                rec["exit"] = int(rec["exit"])
                rec["recorded"] = int(rec["recorded"])
                rows.append(rec)
    return rows


def summarize(rows):
    by_level = {}
    for r in rows:
        by_level.setdefault(r["level"], []).append(r)
    levels = []
    for level in sorted(by_level):
        group = by_level[level]
        durs_ms = [r["dur_ns"] / 1e6 for r in group]
        ok = sum(1 for r in group if r["exit"] == 0 and r["recorded"] == 1)
        pids = {r["pid"] for r in group}
        window_ns = max(r["end_ns"] for r in group) - min(r["start_ns"] for r in group)
        throughput = (ok / (window_ns / 1e9)) if window_ns > 0 else None
        timeouts = sum(1 for r in group if r["dur_ns"] >= 4_900_000_000)
        levels.append({
            "level": level,
            "writer_processes": len(pids),
            "writes": len(group),
            "recorded": ok,
            "busy_timeout_suspects": timeouts,
            "throughput_per_s": round(throughput, 1) if throughput else None,
            "mean_ms": round(statistics.fmean(durs_ms), 2),
            "p50_ms": round(pctl(durs_ms, 0.50), 2),
            "p95_ms": round(pctl(durs_ms, 0.95), 2),
            "p99_ms": round(pctl(durs_ms, 0.99), 2),
            "max_ms": round(max(durs_ms), 2),
        })
    # The sanity gate needs a curve to be meaningful: one level (or none)
    # proves nothing about contention, so it defaults to SOFT. (The
    # envelope-001 single-row collapse passed under the older shape; the
    # measurement face is exactly what the gate exists to catch.)
    soft = True
    if len(levels) >= 2:
        base, top = levels[0], levels[-1]
        if base["mean_ms"] and top["mean_ms"] and top["mean_ms"] >= base["mean_ms"] * 1.05:
            soft = False
    return levels, soft


def load_env(path):
    env = {}
    with open(path) as fh:
        for line in fh:
            if "=" in line:
                k, v = line.strip().split("=", 1)
                env[k] = v
    return env


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--format", choices=("md", "json"), required=True)
    ap.add_argument("--csv", required=True)
    ap.add_argument("--env", required=True)
    ap.add_argument("--verify", required=True)
    args = ap.parse_args()

    rows = read_rows(args.csv)
    levels, soft = summarize(rows)
    env = load_env(args.env)
    verify = {"status": "missing"}
    if os.path.exists(args.verify):
        with open(args.verify) as fh:
            verify = json.load(fh)

    summary = {"env": env, "levels": levels,
               "contention_sanity": "SOFT" if soft else "OK",
               "chain_verify_status": verify.get("status"),
               "total_writes": len(rows)}
    if args.format == "json":
        print(json.dumps(summary, indent=2))
        return
    if not rows:
        raise SystemExit("report: no CSV rows found - the sweep produced no evidence")

    out = ["# B06 spine write envelope - Provisional baseline", ""]
    out.append("## Binding facts")
    for k in sorted(env):
        out.append("- " + k + ": `" + env[k] + "`")
    out.append("")
    out.append("## Curve")
    out.append("| level | writer-procs | writes | recorded | timeout-suspects | thr/s | mean ms | p50 | p95 | p99 | max |")
    out.append("|---|---|---|---|---|---|---|---|---|---|---|")
    for lv in levels:
        out.append("| {level} | {writer_processes} | {writes} | {recorded} | {busy_timeout_suspects} | {throughput_per_s} | {mean_ms} | {p50_ms} | {p95_ms} | {p99_ms} | {max_ms} |".format(**lv))
    out.append("")
    out.append("## Gates")
    out.append("- contention sanity: **" + summary["contention_sanity"] + "**"
               + ("  (latency did not rise with the level - the envelope is NOT verified)" if soft else ""))
    out.append("- activity chain verification after the sweep: **" + str(verify.get("status")) + "**")
    out.append("")
    out.append("## Methodology (declared)")
    out.append("- Writers are real production mc verb executions (the lane driver invokes the same verbs.SignoffRecord path the CLI routes to, with the host identity and OpenSpine's live schema gate); the process arm holds one container per writer (the production docker-exec topology), the thread arm holds N independent spine handles in one container; file-level write-lock contention is handle-scoped, so both arms press the same single writer.")
    out.append("- The histogram is the per-write wall-latency distribution; a busy wait of the full 5s budget surfaces as a timeout suspect (dur >= 4.9s), the right-censored tail of the same histogram. SQLite exposes no internal lock-wait counters to the caller; no kernel-level lock statistics are claimed.")
    out.append("- Envelope status = Provisional (DC section 6). Retest hook: after the G-A through G-F obligations close, rerun this lane unchanged (new RUN_NAME) and diff summary.json levels against this baseline - the gates themselves are write-path cost.")
    out.append("")
    print("\n".join(out))
    if soft:
        raise SystemExit("SANITY SOFT: latency did not rise with concurrency - inspect before believing the envelope")


if __name__ == "__main__":
    main()
