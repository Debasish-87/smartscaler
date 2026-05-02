set -euo pipefail

VERSION="${VERSION:-dev}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
IMAGE="smartscaler:${VERSION}"
NAMESPACE_OPERATOR="kube-system"
NAMESPACE_WORKLOAD="default"

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CRD_FILE="$BASE_DIR/deploy/crd/crd.yaml"
RBAC_FILE="$BASE_DIR/deploy/rbac/rbac.yaml"
OPERATOR_FILE="$BASE_DIR/deploy/operator/operator.yaml"
WORKLOAD_FILE="$BASE_DIR/deploy/workloads/deployment.yaml"
SCALER_FILE="$BASE_DIR/deploy/samples/scaler.yaml"

log() { echo -e "\n\033[1;34m[$1]\033[0m $2"; }
ok()  { echo -e "\033[1;32m✓\033[0m $1"; }
err() { echo -e "\033[1;31m✗\033[0m $1" >&2; exit 1; }

# Prerequisite check
for cmd in kubectl docker minikube; do
  command -v "$cmd" >/dev/null || err "Missing required tool: $cmd"
done

# Clean mode
if [[ "${1:-}" == "clean" ]]; then
  log "CLEAN" "Removing all SmartScaler resources"
  kubectl delete -f "$SCALER_FILE"   --ignore-not-found || true
  kubectl delete -f "$WORKLOAD_FILE" --ignore-not-found || true
  kubectl delete -f "$OPERATOR_FILE" --ignore-not-found || true
  kubectl delete -f "$RBAC_FILE"     --ignore-not-found || true
  kubectl delete -f "$CRD_FILE"      --ignore-not-found || true
  ok "Cleanup complete"
  exit 0
fi

# Cluster setup
log "MINIKUBE" "Ensuring cluster is running"
minikube start --memory=4096 --cpus=4 --driver=docker >/dev/null 2>&1 || true
minikube addons enable metrics-server >/dev/null 2>&1 || true

# Build image
log "BUILD" "Building image: $IMAGE"
eval "$(minikube docker-env)"
docker build \
  --build-arg VERSION="$VERSION" \
  --build-arg COMMIT="$COMMIT" \
  --build-arg BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -t "$IMAGE" \
  "$BASE_DIR"
ok "Image built: $IMAGE"

# Deploy CRD
log "CRD" "Applying CRD"
kubectl apply -f "$CRD_FILE"
kubectl wait --for=condition=Established crd/smartscalers.autoscale.mycompany --timeout=30s
ok "CRD established"

# Deploy RBAC
log "RBAC" "Applying RBAC"
kubectl apply -f "$RBAC_FILE"
ok "RBAC applied"

# Deploy Operator
log "OPERATOR" "Deploying SmartScaler operator"
sed "s|smartscaler:latest|$IMAGE|g" "$OPERATOR_FILE" | kubectl apply -f -
kubectl -n "$NAMESPACE_OPERATOR" rollout status deployment/smartscaler-operator --timeout=120s
ok "Operator is running"

# Deploy workload
log "WORKLOAD" "Deploying demo application"
kubectl -n "$NAMESPACE_WORKLOAD" apply -f "$WORKLOAD_FILE"
kubectl -n "$NAMESPACE_WORKLOAD" rollout status deployment/demo-app --timeout=60s
ok "demo-app is running"

# Apply SmartScaler CR
log "SCALER" "Applying SmartScaler custom resource"
kubectl -n "$NAMESPACE_WORKLOAD" apply -f "$SCALER_FILE"
ok "SmartScaler resource created"

# Status output
log "STATUS" "Current cluster state"
kubectl -n "$NAMESPACE_OPERATOR" get pods -l app=smartscaler
echo ""
kubectl -n "$NAMESPACE_WORKLOAD" get pods
echo ""
kubectl -n "$NAMESPACE_WORKLOAD" get smartscalers -o wide
echo ""
echo "Metrics endpoint:"
echo "$(minikube service smartscaler-metrics -n kube-system --url 2>/dev/null || echo 'kubectl port-forward -n kube-system svc/smartscaler-metrics 8080:8080')/metrics"