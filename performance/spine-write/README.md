# B06 spine write contention lane

The DC §1.1①/§1.1③ obligation made concrete: a multi-process write load
lane over the real spine, and the lock-wait histogram the contract names.
The envelope it records is **Provisional** (DC §6): the first baseline, run
on the O3 hardware, bound by the facts the report prints.

## What it measures

One writer = one real production `mc` verb execution on the spine's write
path: WAL, `busy_timeout(5000)`, `BEGIN IMMEDIATE` single writer, the guard
plane, and the post-B10 chain append (row + chain event + mapping in one
transaction). The chosen write verb is `signoff record` (generic class) -
the heaviest steady-state multi-row append the spine owns today, and the
most recent gate to have landed on the write path.

The driver (`mc/spinewrite`) never spawns programs: it calls the same verb
functions the `mc` command routes to, through the same `OpenSpine` gate.
The lane script then sweeps each concurrency level in two forms:

- **Process arm** — N containers, one writer process each: the literal
  multi-process fact, matching the production topology where every writer
  is a separate `mc` process (host CLI, helper `__system-state`, docker
  exec).
- **Thread arm** — one container holding N independent spine handles
  (N `OpenSpine` connections, N threads): the file-level lock contention
  is handle-scoped, so it presses the same single writer. The two arms
  cross-check each other; a divergence is a methodology finding.

## The histogram, honestly declared

SQLite exposes no per-lock wait counters to a caller. The lock-wait
histogram is therefore the **per-write wall-latency distribution**: the
queue wait dominates it under contention, and a busy wait that consumes
the whole 5s budget surfaces as a right-censored tail (the report counts
`dur >= 4.9s` rows as timeout suspects). No kernel-level lock statistics
are claimed anywhere in this lane.

## Run

Prerequisite (host, Linux): docker, plus `$LANE_DIR/bin` holding the
cross-built linux/amd64 `mc` and `spine-write-driver`:

```sh
cd mc && GOOS=linux GOARCH=amd64 go build -o "$LANE_DIR/bin/mc" ./cmd/mc && \
GOOS=linux GOARCH=amd64 go build -o "$LANE_DIR/bin/spine-write-driver" ./spinewrite
LANE_DIR=/root/b06-lane IMAGE=alpine:3.20 BUILD_HEAD="$(git rev-parse HEAD)" \
LEVELS=1,8,32,64 TRIALS=100 ./performance/spine-write/run.sh
```

`run.sh` records the machine/build/schema binding facts (`env.txt`),
provisions a fresh scratch spine (`--mode provision`), seeds one lane user
and session through the real identity verbs, sweeps the levels, verifies
the `activity` chain with the read-only `verify-chain` path (`--mode
verify`; a broken chain stops the lane - that is a defect, never a tuned
number), and writes `results/<RUN_NAME>/{env.txt,verify.json,csv/*.csv,
summary.json,report.md}`.

## Gates

- **S2 stop**: the sweep ends with a full `activity` chain verification; a
  non-`ok` status exits red. Contention pressure that corrupts the chain is
  a defect report, not a data point.
- **contention sanity**: the report builder refuses the `OK` label when
  mean latency does not rise from the baseline level to the top level - an
  envelope whose fan-out never actually queued is `SOFT` and says so.

## Retest hook (DC §6 closure obligation)

After the G-A through G-F obligations close, rerun this lane **unchanged**
with a new `RUN_NAME` and diff `summary.json` levels against the baseline
here: the gates are themselves write-path cost, so the envelope moves. The
hook is this README plus the AM §4b G-row annotation; the retest is owned
by whoever closes G, and the baseline diff is its evidence.

## Scope edges

- One spine, one host (the platform topology); multi-instance is out of
  scope by contract.
- The measured verb shape is one write class; tick-mixed or attachment-
  heavy shapes would be a separate lane, not a widening of this one.
- The scratch spine is a lane artifact under `$LANE_DIR/spine`; production
  spines are never inputs.
