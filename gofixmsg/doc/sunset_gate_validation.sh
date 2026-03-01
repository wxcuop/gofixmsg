#!/bin/bash

# Sunset Gate Validation Script
# Verifies all criteria for deprecating Python runtime
# Exit code: 0 = all gates pass, >0 = at least one gate failed

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Track gate status
GATES_PASSED=0
GATES_FAILED=0

print_header() {
    echo -e "${BLUE}=== $1 ===${NC}\n"
}

pass_gate() {
    echo -e "${GREEN}✓ PASS${NC}: $1\n"
    ((GATES_PASSED++))
}

fail_gate() {
    echo -e "${RED}✗ FAIL${NC}: $1\n"
    ((GATES_FAILED++))
}

warn_gate() {
    echo -e "${YELLOW}⚠ WARN${NC}: $1\n"
}

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOFIXMSG_DIR="$SCRIPT_DIR/gofixmsg"
ZARCHIVE_DIR="$SCRIPT_DIR/zz_archive"

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  SUNSET GATE VALIDATION SCRIPT         ║${NC}"
echo -e "${BLUE}║  GoFixMsg Migration Readiness Check    ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}\n"

# Gate 1: Go tests passing
print_header "Gate 1: Go Test Suite"
if cd "$GOFIXMSG_DIR" && go test ./... -v -timeout 60s 2>&1 | tail -20; then
    pass_gate "All Go tests passing"
else
    fail_gate "Go tests failed - run: cd gofixmsg && go test ./..."
fi

# Gate 2: Integration tests
print_header "Gate 2: Integration Tests"
if cd "$GOFIXMSG_DIR" && go test ./integration/... -v -timeout 60s 2>&1 | tail -20; then
    pass_gate "Integration tests passing"
else
    fail_gate "Integration tests failed - run: cd gofixmsg && go test ./integration/..."
fi

# Gate 3: Build verification
print_header "Gate 3: Build Verification"
if cd "$GOFIXMSG_DIR" && go build ./... 2>&1; then
    pass_gate "All packages build successfully"
else
    fail_gate "Build failed - run: cd gofixmsg && go build ./..."
fi

# Gate 4: Examples compilation
print_header "Gate 4: Examples Compilation"
cd "$GOFIXMSG_DIR/examples"
if go build -o /tmp/acceptor_test ./acceptor/main.go 2>&1 && \
   go build -o /tmp/initiator_test ./initiator/main.go 2>&1; then
    rm -f /tmp/acceptor_test /tmp/initiator_test
    pass_gate "Example binaries compile successfully"
else
    fail_gate "Example binaries failed to compile"
fi

# Gate 5: Migration documentation
print_header "Gate 5: Migration Documentation"
if [ -f "$GOFIXMSG_DIR/doc/migration.md" ]; then
    pass_gate "migration.md exists"
else
    fail_gate "migration.md not found"
fi

if [ -f "$GOFIXMSG_DIR/doc/sunset_recommendation.md" ]; then
    pass_gate "sunset_recommendation.md exists"
else
    fail_gate "sunset_recommendation.md not found"
fi

# Gate 6: Examples documentation
print_header "Gate 6: Examples Documentation"
if [ -f "$GOFIXMSG_DIR/examples/README.md" ]; then
    pass_gate "Examples README.md exists"
else
    fail_gate "Examples README.md not found"
fi

# Gate 7: Deprecation notice
print_header "Gate 7: Deprecation Notice"
if [ -f "$SCRIPT_DIR/DEPRECATED.md" ]; then
    pass_gate "DEPRECATED.md published at root"
else
    fail_gate "DEPRECATED.md not found at root"
fi

# Gate 8: Python code archived
print_header "Gate 8: Python Code Archive"
if [ -d "$ZARCHIVE_DIR/pyfixmsg" ] && \
   [ -d "$ZARCHIVE_DIR/pyfixmsg_plus" ]; then
    pass_gate "Python code archived in zz_archive/"
else
    fail_gate "Python code not properly archived"
fi

# Gate 9: Project structure
print_header "Gate 9: Project Structure"
if [ -f "$GOFIXMSG_DIR/go.mod" ] && \
   [ -d "$GOFIXMSG_DIR/engine" ] && \
   [ -d "$GOFIXMSG_DIR/fixmsg" ] && \
   [ -d "$GOFIXMSG_DIR/network" ]; then
    pass_gate "GoFixMsg project structure valid"
else
    fail_gate "GoFixMsg project structure incomplete"
fi

# Gate 10: Config.ini present
print_header "Gate 10: Configuration Template"
if [ -f "$GOFIXMSG_DIR/examples/config.ini" ]; then
    pass_gate "Example config.ini available"
else
    fail_gate "Example config.ini not found"
fi

# Summary
print_header "SUNSET GATE SUMMARY"

TOTAL=$((GATES_PASSED + GATES_FAILED))
echo "Results: ${GREEN}$GATES_PASSED passed${NC}, ${RED}$GATES_FAILED failed${NC} (Total: $TOTAL)"
echo ""

if [ $GATES_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ ALL GATES PASSED - Python runtime is ready for deprecation${NC}\n"
    echo "Next steps:"
    echo "  1. Review DEPRECATED.md"
    echo "  2. Notify users of deprecation"
    echo "  3. Point to migration guide: gofixmsg/doc/migration.md"
    echo "  4. Archive Python repo or mark as read-only"
    exit 0
else
    echo -e "${RED}✗ GATES FAILED - Address failures before deprecation${NC}\n"
    echo "Failed gates:"
    echo "  - Review error messages above"
    echo "  - Run individual tests manually for details"
    echo "  - Fix issues and re-run this script"
    exit 1
fi
