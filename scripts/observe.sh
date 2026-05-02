set -euo pipefail

NAMESPACE="${1:-default}"
INTERVAL="${2:-3}"

clear
exec watch -n "$INTERVAL" "
echo '╔═══════════════════════════════════════════════════════╗'
echo '║           SMARTSCALER LIVE DASHBOARD                  ║'
echo '╚═══════════════════════════════════════════════════════╝'
echo ''
echo '── SmartScalers (all namespaces) ──────────────────────'
kubectl get smartscalers -A -o wide 2>/dev/null

echo ''
echo '── Pods ────────────────────────────────────────────────'
kubectl get pods -n $NAMESPACE 2>/dev/null

echo ''
echo '── CPU (pods) ──────────────────────────────────────────'
kubectl top pods -n $NAMESPACE 2>/dev/null || echo '  (metrics-server not ready yet)'

echo ''
echo '── CPU (nodes) ─────────────────────────────────────────'
kubectl top nodes 2>/dev/null || echo '  (metrics-server not ready yet)'

echo ''
echo '── Operator logs (last 8 lines) ────────────────────────'
kubectl logs -n kube-system -l app=smartscaler --tail=8 2>/dev/null

echo ''
echo '── Recent events ───────────────────────────────────────'
kubectl get events -n $NAMESPACE --sort-by=.metadata.creationTimestamp 2>/dev/null | tail -6
"
