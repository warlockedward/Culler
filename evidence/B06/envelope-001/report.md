# B06 spine write envelope - Provisional baseline

## Binding facts
- build_head: `a23d91c`
- cpu: `128`
- date_utc: `2026-11-27T02:39:22Z`
- disk_free: `20G`
- disk_used_pct: `96%`
- image: `alpine:3.22`
- image_id: `sha256:49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f`
- kernel: `Linux 5.15.0-94-generic x86_64`
- levels: `1,8,32,64 trials=100 proc_arm=1`
- mem_gib: `503`
- sha256_driver: `fd01bdb4bad720398a8f8a203cfefa7b240cf6d8780d66fc98d7a704a7b6fbd7`
- sha256_mc: `5ca5dcae6326d501448381c0b688ec37258ba95c9eed31363d181b6c50899c7e`

## Curve
| level | writer-procs | writes | recorded | timeout-suspects | thr/s | mean ms | p50 | p95 | p99 | max |
|---|---|---|---|---|---|---|---|---|---|---|
| 64 | 1 | 6400 | 6399 | 3 | 278.1 | 200.21 | 8.17 | 1239.1 | 2438.1 | 5040.82 |

## Gates
- contention sanity: **OK**
- activity chain verification after the sweep: **ok**

## Methodology (declared)
- Writers are real production mc verb executions (the lane driver invokes the same verbs.SignoffRecord path the CLI routes to, with the host identity and OpenSpine's live schema gate); the process arm holds one container per writer (the production docker-exec topology), the thread arm holds N independent spine handles in one container; file-level write-lock contention is handle-scoped, so both arms press the same single writer.
- The histogram is the per-write wall-latency distribution; a busy wait of the full 5s budget surfaces as a timeout suspect (dur >= 4.9s), the right-censored tail of the same histogram. SQLite exposes no internal lock-wait counters to the caller; no kernel-level lock statistics are claimed.
- Envelope status = Provisional (DC section 6). Retest hook: after the G-A through G-F obligations close, rerun this lane unchanged (new RUN_NAME) and diff summary.json levels against this baseline - the gates themselves are write-path cost.

