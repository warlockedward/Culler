#!/bin/bash
# A36-F1 / A31-F1(3) production arms on 218. Run AFTER deploy activation.
# R-a note: gateway-allow is the PRODUCT code path (shz binary), pre-approved
# in the deploy plan (plan-001 §2 P5 + owner ruling round-63). Revoke same
# round = F-2 negative control.
set -e
SHZ=/root/.local/bin/shz
export MC_HOME=/data-shunhong/shorizon-prod
D=/data-shunhong/shorizon-deploy-r63

echo "== A0 proxy bind (product surface)"
$SHZ gateway status 2>&1 | head -12

# Discovery: the product panel prints the proxy port; fall back to the
# resident service tree, always excluding the dashboard surfaces.
PROXY_PORT=$($SHZ gateway status 2>/dev/null | grep -oE 'port: *[0-9]+' | grep -oE '[0-9]+' | head -1)
if ! printf '%s' "$PROXY_PORT" | grep -Eq '^[0-9]{2,5}$'; then
  PROXY_PORT=""
  UNIT=$(systemctl list-units --type=service --no-legend --full 2>/dev/null | awk '{print $1}' | grep -E '^shorizon(-[0-9a-f]{12})?-resident' | head -1 || true)
  if [ -n "$UNIT" ]; then
    MAIN=$(systemctl show "$UNIT" -p MainPID --value 2>/dev/null || true)
    if [ -n "${MAIN:-}" ] && [ "$MAIN" != "0" ]; then
      TREE="$MAIN $(ps --ppid "$MAIN" -o pid= | tr -d ' ')"
      for pid in $TREE; do
        for found in $(ss -tlnp 2>/dev/null | grep "pid=$pid," | grep -oE '127\.0\.0\.1:[0-9]+' | cut -d: -f2); do
          case "$found" in 7333|7334|7335) continue ;; esac
          PROXY_PORT="$found"; break 2
        done
      done
    fi
  fi
fi
if ! printf '%s' "$PROXY_PORT" | grep -Eq '^[0-9]{2,5}$'; then
  echo "NO numeric proxy port discovered - arms cannot run; report"; exit 3
fi
echo "proxy port=$PROXY_PORT"

echo "== snapshot ufw before"
ufw status numbered > "$D/ufw-f1-before.txt"

echo "== A1 gateway allow (scoped rule via product code)"
$SHZ gateway allow --port "$PROXY_PORT" 2>&1 | tail -6

# Constant command strings only; the numeric port enters as a positional
# argument, never concatenated into the shell text.
echo "== A2 real container dial (production-shape flags)"
docker run --rm --add-host host.docker.internal:host-gateway alpine:3.22 \
  sh -c 'nc -w3 -z host.docker.internal "$1" && echo DIAL-OK || echo DIAL-FAIL' sh "$PROXY_PORT"

echo "== A3 revoke + negative control (F-2)"
$SHZ gateway revoke 2>&1 | tail -3
docker run --rm --add-host host.docker.internal:host-gateway alpine:3.22 \
  sh -c 'nc -w3 -z host.docker.internal "$1" && echo STILL-REACHABLE-BAD || echo REVOKED-TIMEOUT-GOOD' sh "$PROXY_PORT"

echo "== A4 ufw diff vs before (expect empty)"
ufw status numbered > "$D/ufw-f1-after.txt"
diff "$D/ufw-f1-before.txt" "$D/ufw-f1-after.txt" && echo ZERO-RESIDUE

echo "== A5 main daemon foreign tenants untouched"
docker ps --format "{{.Names}}" | sort | md5sum
