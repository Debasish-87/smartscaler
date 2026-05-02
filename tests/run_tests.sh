set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ "$SCRIPT_DIR" == */tests ]]; then
  PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
else
  PROJECT_ROOT="$SCRIPT_DIR"
fi
cd "$PROJECT_ROOT"

BOLD='\033[1m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}═══════════════════════════════════════════════${NC}"
echo -e "${BOLD}        SmartScaler Test Suite${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════${NC}"
echo -e "  Root: ${PROJECT_ROOT}"

FAILED=0

run_go_test() {
  local label="$1"
  local path="$2"
  local timeout="${3:-30s}"

  echo -e "\n${BOLD}── $label ─────────────────────────────────${NC}"

  if go test -v -race -count=1 -timeout="$timeout" "$path"; then
    echo -e "${GREEN}✓ $label passed${NC}"
  else
    echo -e "${RED}✗ $label FAILED${NC}"
    FAILED=$((FAILED + 1))
  fi
}

# ── Unit Tests ────────────────────────────────────────────────
run_go_test "Decision Logic"  "./tests/pkg/decision/..."
run_go_test "Cost Analysis"   "./tests/pkg/cost/..."
run_go_test "Utilities"       "./tests/pkg/utils/..."
run_go_test "Scaler Core"     "./tests/pkg/scaler/..."

# ── Regression Tests ──────────────────────────────────────────
run_go_test "Regression"      "./tests/pkg/regression/..."

# ── Integration Tests ─────────────────────────────────────────
run_go_test "Integration"     "./tests/pkg/integration/..." "60s"

# ── Coverage ──────────────────────────────────────────────────
echo -e "\n${BOLD}── Full Suite (with coverage) ─────────────────${NC}"

go test -race -count=1 -timeout=120s \
  -coverprofile=coverage.out \
  -covermode=atomic \
  -coverpkg=./pkg/... \
  ./tests/...

echo ""
echo -e "${BOLD}  Per-package breakdown:${NC}"
go tool cover -func=coverage.out \
  | grep -E "\.(go):" \
  | awk '{printf "  %-55s %s\n", $1, $3}'

echo ""
TOTAL=$(go tool cover -func=coverage.out | grep "^total" | awk '{print $3}')
echo -e "${YELLOW}${BOLD}  Total Coverage: ${TOTAL}${NC}"

# ── HTML Report ───────────────────────────────────────────────
go tool cover -html=coverage.out -o coverage.html
echo -e "  coverage.html → xdg-open coverage.html"

# ── Final Result ──────────────────────────────────────────────
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════${NC}"
if [ "$FAILED" -eq 0 ]; then
  echo -e "${GREEN}${BOLD}  ✓ All tests passed! Coverage: ${TOTAL}${NC}"
else
  echo -e "${RED}${BOLD}  ✗ ${FAILED} suite(s) failed${NC}"
  exit 1
fi
echo -e "${BLUE}═══════════════════════════════════════════════${NC}"
echo ""