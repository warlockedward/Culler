# B06 spine write envelope - Provisional baseline

## Binding facts
- build_head: `2f07dec`
- cpu: `128`
- date_utc: `2026-11-27T02:47:53Z`
- disk_free: `21G`
- disk_used_pct: `96%`
- image: `alpine:3.22`
- image_id: `sha256:49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f`
- kernel: `Linux 5.15.0-94-generic x86_64`
- levels: `1,8,32,64 trials=100 proc_arm=1`
- mem_gib: `503`
- sha256_driver: `37bebce93ff66cea10ed3b6061b8b6e40e6e5316be7b2c17d214dc16df7cf910`
- sha256_mc: `5ca5dcae6326d501448381c0b688ec37258ba95c9eed31363d181b6c50899c7e`

## Curve
| level | writer-procs | writes | recorded | timeout-suspects | thr/s | mean ms | p50 | p95 | p99 | max |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 1 | 200 | 200 | 0 | 115.5 | 2.59 | 2.32 | 4.28 | 9.86 | 13.23 |
| 8 | 1 | 1600 | 1600 | 0 | 221.9 | 10.81 | 1.98 | 20.16 | 184.44 | 1135.04 |
| 32 | 1 | 6400 | 6400 | 0 | 262.8 | 54.09 | 2.4 | 193.66 | 1435.15 | 4336.57 |
| 64 | 1 | 12800 | 12798 | 3 | 242.8 | 120.35 | 3.26 | 819.02 | 2063.75 | 5338.7 |

## Gates
- contention sanity: **OK**
- activity chain verification after the sweep: **ok**

## Methodology (declared)
- Writers are real production mc verb executions (the lane driver invokes the same verbs.SignoffRecord path the CLI routes to, with the host identity and OpenSpine's live schema gate); the process arm holds one container per writer (the production docker-exec topology), the thread arm holds N independent spine handles in one container; file-level write-lock contention is handle-scoped, so both arms press the same single writer.
- The histogram is the per-write wall-latency distribution; a busy wait of the full 5s budget surfaces as a timeout suspect (dur >= 4.9s), the right-censored tail of the same histogram. SQLite exposes no internal lock-wait counters to the caller; no kernel-level lock statistics are claimed.
- Envelope status = Provisional (DC section 6). Retest hook: after the G-A through G-F obligations close, rerun this lane unchanged (new RUN_NAME) and diff summary.json levels against this baseline - the gates themselves are write-path cost.

