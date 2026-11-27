#!/bin/sh
# B06 spine write contention lane — host orchestrator for the O3 envelope.
#
# Prerequisite: $LANE_DIR/bin holds the cross-built linux/amd64 `mc` and
# `spine-write-driver`; the lane runs each writer as its own container (the
# literal multi-process arm, the production docker-exec topology) with one
# spine handle per process; the thread arm (single container, handles=level)
# is selectable for cross-checking the file-lock equivalence claim.
#
# Usage:
#   LANE_DIR=/root/b06-lane IMAGE=alpine:3.20 BUILD_HEAD=<sha> LEVELS=1,8,32,64 TRIALS=100 ./run.sh
set -eu
cd "$(dirname "$0")"

LANE_DIR="${LANE_DIR:?LANE_DIR is required}"
IMAGE="${IMAGE:-alpine:3.20}"
LEVELS="${LEVELS:-1,8,32,64}"
TRIALS="${TRIALS:-100}"
PROC_ARM="${PROC_ARM:-1}" # 1 = one container per writer; thread arm always runs too
RUN_NAME="${RUN_NAME:-b06-envelope}"
OUT="$LANE_DIR/results/$RUN_NAME"
mkdir -p "$OUT" "$LANE_DIR/lane/csv" "$LANE_DIR/spine"

mounts() {
  printf '%s' "-v $LANE_DIR/spine:/mc/spine -v $LANE_DIR/lane:/mc/lane -v $LANE_DIR/bin/mc:/usr/local/bin/mc:ro -v $LANE_DIR/bin/spine-write-driver:/usr/local/bin/spine-write-driver:ro"
}

# container runs one driver mode inside the helper-scope mount set.
container() {
  # shellcheck disable=SC2046
  docker run --rm $(mounts) "$IMAGE" /usr/local/bin/spine-write-driver "$@"
}

echo "== env facts =="
{
  echo "date_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "image=$IMAGE"
  echo "image_id=$(docker image inspect --format '{{.Id}}' "$IMAGE")"
  echo "kernel=$(uname -srm)"
  echo "cpu=$(nproc)"
  echo "mem_gib=$(free -g | awk '/^Mem:/{print $2}')"
  echo "disk_free=$(df -h "$LANE_DIR" | awk 'NR==2{print $4}')"
  echo "disk_used_pct=$(df -h "$LANE_DIR" | awk 'NR==2{print $5}')"
  echo "build_head=${BUILD_HEAD:-unknown}"
  echo "sha256_mc=$(sha256sum "$LANE_DIR/bin/mc" | cut -d' ' -f1)"
  echo "sha256_driver=$(sha256sum "$LANE_DIR/bin/spine-write-driver" | cut -d' ' -f1)"
  echo "levels=$LEVELS trials=$TRIALS proc_arm=$PROC_ARM"
} | tee "$OUT/env.txt"

# capture runs one lane step, saves its output, and fails the lane on a
# non-zero exit — never through a pipe, whose status would mask the step's
# own (the envelope-002 abort: a failed provision sailed on behind `| tee`).
capture() {
  out="$1"; shift
  if "$@" > "$out" 2>&1; then
    cat "$out"
  else
    status=$?
    cat "$out" >&2
    echo "lane step failed (exit $status): $*" >&2
    exit 1
  fi
}

# The lane spine is scratch: wipe the lane-owned files so every envelope
# starts from a freshly initialized identity, never from a leftover chain.
rm -f "$LANE_DIR"/spine/spine.db "$LANE_DIR"/spine/spine.db-wal "$LANE_DIR"/spine/spine.db-shm
rm -f "$LANE_DIR"/lane/csv/*.csv "$LANE_DIR"/lane/token "$LANE_DIR"/lane/seqbase "$LANE_DIR"/lane/user.json

echo "== provision =="
capture "$OUT/provision.txt" container --mode provision
echo "== seed =="
capture "$OUT/seed.txt" container --mode seed

# level N: the process arm forks N containers in the background (literal
# multi-process writers, one handle each); the thread arm is one container
# holding N handles. Both write pid-named CSV shards into the shared volume.
for level in $(echo "$LEVELS" | tr ',' ' '); do
  echo "== level $level: process arm =="
  if [ "$PROC_ARM" = "1" ]; then
    pids=""
    i=1
    while [ "$i" -le "$level" ]; do
      container --mode writer --level "$level" --trials "$TRIALS" --handles 1 &
      pids="$pids $!"
      i=$((i + 1))
    done
    for p in $pids; do
      wait "$p"
    done
  fi
  echo "== level $level: thread arm =="
  container --mode writer --level "$level" --trials "$TRIALS"
done

echo "== verify =="
capture "$OUT/verify.json" container --mode verify

mkdir -p "$OUT/csv"
mv "$LANE_DIR"/lane/csv/*.csv "$OUT/csv/" 2>/dev/null || true
REPORT_TOOL="$(dirname "$0")/report.py"
python3 "$REPORT_TOOL" --format json --csv "$OUT/csv" --env "$OUT/env.txt" \
  --verify "$OUT/verify.json" > "$OUT/summary.json"
python3 "$REPORT_TOOL" --format md --csv "$OUT/csv" --env "$OUT/env.txt" \
  --verify "$OUT/verify.json" > "$OUT/report.md"
echo "== report: $OUT/report.md =="
