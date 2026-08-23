#!/usr/bin/env bash
# Create out-of-band secrets for deploybot (not managed by Argo CD).
#
#   export DEPLOYBOT_GITHUB_TOKEN=...   # read:packages + repo (kcloud-ops)
#   export GHCR_TOKEN=...               # optional if ghcr-auth already exists
#   ./gen-secrets.sh                    # CONTEXT=homelab or prod-sjc1
set -euo pipefail

NAMESPACE=${NAMESPACE:-deploybot-system}
CONTEXT=${CONTEXT:-homelab}

: "${DEPLOYBOT_GITHUB_TOKEN:?set DEPLOYBOT_GITHUB_TOKEN}"

kubectl --context="$CONTEXT" -n "$NAMESPACE" create secret generic deploybot-env \
  --from-literal=DEPLOYBOT_GITHUB_TOKEN="$DEPLOYBOT_GITHUB_TOKEN" \
  --from-literal=DEPLOYBOT_GIT_TOKEN="${DEPLOYBOT_GIT_TOKEN:-$DEPLOYBOT_GITHUB_TOKEN}" \
  --dry-run=client -o yaml | kubectl --context="$CONTEXT" apply -f -

if ! kubectl --context="$CONTEXT" -n "$NAMESPACE" get secret ghcr-auth >/dev/null 2>&1; then
  if [[ -n "${GHCR_TOKEN:-}" ]]; then
    kubectl --context="$CONTEXT" -n "$NAMESPACE" create secret docker-registry ghcr-auth \
      --docker-server=https://ghcr.io \
      --docker-username="${GHCR_USER:-ianunruh}" \
      --docker-password="$GHCR_TOKEN" \
      --dry-run=client -o yaml | kubectl --context="$CONTEXT" apply -f -
  else
    echo "ghcr-auth missing — copy from kmc-system or play, or set GHCR_TOKEN:" >&2
    echo "  kubectl --context=$CONTEXT -n kmc-system get secret ghcr-auth -o yaml \\" >&2
    echo "    | sed 's/namespace: kmc-system/namespace: $NAMESPACE/' \\" >&2
    echo "    | kubectl --context=$CONTEXT apply -f -" >&2
  fi
fi

# Bound token so this cluster's deploybot can talk to the peer apiserver.
# Re-run annually; TokenRequest max duration is 8760h. Peer SA must exist
# (sync the deploybot app on the other cluster first, then re-run).
mint_peer() {
  local peer_context=$1 peer_key=$2
  if ! kubectl --context="$peer_context" -n "$NAMESPACE" get sa deploybot >/dev/null 2>&1; then
    echo "peer SA $NAMESPACE/deploybot missing on $peer_context — sync deploybot there, then re-run" >&2
    return 0
  fi
  echo "Minting $peer_context deploybot SA token..."
  PEER_TOKEN=$(kubectl --context="$peer_context" -n "$NAMESPACE" create token deploybot --duration=8760h)
  kubectl --context="$CONTEXT" -n "$NAMESPACE" create secret generic deploybot-cluster-tokens \
    --from-literal="$peer_key"="$PEER_TOKEN" \
    --dry-run=client -o yaml | kubectl --context="$CONTEXT" apply -f -
}

if [[ "$CONTEXT" == "homelab" ]]; then
  mint_peer prod-sjc1 prod
elif [[ "$CONTEXT" == "prod-sjc1" ]]; then
  mint_peer homelab homelab
fi

echo "✓ secrets applied in $NAMESPACE (context $CONTEXT)"
