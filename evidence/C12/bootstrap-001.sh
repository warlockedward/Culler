#!/bin/bash
# C12 transitional bootstrap: store ONE api-key connection through the product
# verb (same body the dashboard uses), key entered by the operator at their own
# terminal via hidden input. The key never touches argv, files, or logs; the
# verb reads it from stdin only.
set -eu
export MC_HOME=/data-shunhong/shorizon-prod
SHZ=/root/.local/bin/shz

echo "This stores the API key for the [anthropic-key] agent profile connection"
echo "(claude-api-key credential) into the host secret store of this deployment."
printf 'Credential id from config.toml (claude-api-key | codex-api-key | openrouter): '
read -r NAME
printf 'Template id (same as credential id for api-key kinds; opencode:<provider-id> for catalog providers): '
read -r TEMPLATE

KEY=""
while [ -z "$KEY" ]; do
  printf 'Paste the API key (input hidden, empty entry repeats): '
  read -rs KEY
  printf '\n'
done

UUID=$(cat /proc/sys/kernel/random/uuid)

# Build the JSON on stdin of mc via a heredoc pipe: printf of secret goes only
# into the child's stdin, never into an argument or a file.
{
  printf '{"submission_id":"%s","revision":"","id":"%s","name":"%s","template_id":"%s","api_key":"' "$UUID" "$NAME" "$NAME" "$TEMPLATE"
  printf '%s' "$KEY"
  printf '"}\n'
} | mc agent-profile connection-save > "/data-shunhong/shorizon-deploy-r63/bootstrap-$NAME.out" 2>&1 && echo "STORE-OK: credential saved for $NAME" || {
  echo "STORE-FAILED (diagnostic below, key is never echoed by the product):"
  tail -5 "/data-shunhong/shorizon-deploy-r63/bootstrap-$NAME.out"
  exit 4
}
unset KEY
