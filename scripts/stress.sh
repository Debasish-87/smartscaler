set -euo pipefail

MODE="${1:-high}"
DURATION="${2:-120}"
NAMESPACE="${3:-default}"
DEPLOYMENT="demo-app"

if [ "$MODE" != "high" ] && [ "$MODE" != "low" ]; then
  echo "[ERROR] Invalid mode: '${MODE}'. Use 'high' or 'low'."
  exit 1
fi

echo "[STRESS] Mode=${MODE} | Duration=${DURATION}s | Namespace=${NAMESPACE}"
echo "[STRESS] Setting LOAD_INTENSITY=${MODE} on deployment/${DEPLOYMENT}..."

kubectl -n "$NAMESPACE" set env deployment/"$DEPLOYMENT" \
  LOAD_INTENSITY="${MODE}"

echo "[STRESS] Rolling update has started — pods are restarting..."

kubectl -n "$NAMESPACE" rollout status deployment/"$DEPLOYMENT" --timeout=60s \
  && echo "[STRESS] Deployment ready — load=${MODE} is active!" \
  || echo "[WARN] Rollout timed out — check: kubectl describe deployment ${DEPLOYMENT}"

echo ""
echo "[STRESS] Expected behavior:"
if [ "$MODE" = "high" ]; then
  echo "  CPU per pod: ~170–190m  (threshold: 120m)"
  echo "  Action:      scale_up → max 10 replicas"
else
  echo "  CPU per pod: ~40–60m   (threshold: 60m)"
  echo "  Action:      scale_down → min 2 replicas"
fi

echo ""
echo "[STRESS] Monitor using:"
echo "  watch -n 2 kubectl get smartscalers -o wide"
echo ""
echo "[STRESS] Operator logs:"
echo "  kubectl logs -n kube-system -l app=smartscaler -f"

if [ "$MODE" = "high" ] && [ "$DURATION" -gt 0 ]; then
  echo ""
  echo "[STRESS] Will automatically reset to 'low' after ${DURATION}s..."
  (
    sleep "$DURATION"
    echo "[STRESS] Duration completed — resetting LOAD_INTENSITY=low..."
    kubectl -n "$NAMESPACE" set env deployment/"$DEPLOYMENT" LOAD_INTENSITY=low
    echo "[STRESS] Reset complete."
  ) &
  echo "[STRESS] Reset process running in background (PID: $!)"
fi