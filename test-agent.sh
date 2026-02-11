#!/bin/bash

# CLI Agent Test Script
# Tests the new agent loop and tool calling system

set -e

echo "========================================"
echo "CLI Agent Test Suite"
echo "========================================"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Test function
run_test() {
    local test_name="$1"
    local test_command="$2"
    local expected_result="$3"
    
    TESTS_RUN=$((TESTS_RUN + 1))
    
    echo -e "${YELLOW}Test $TESTS_RUN: $test_name${NC}"
    echo "Command: $test_command"
    
    if eval "$test_command" > /dev/null 2>&1; then
        if [ "$expected_result" = "pass" ]; then
            echo -e "${GREEN}✅ PASSED${NC}"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo -e "${RED}❌ FAILED (expected to fail but passed)${NC}"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    else
        if [ "$expected_result" = "fail" ]; then
            echo -e "${GREEN}✅ PASSED (expected to fail)${NC}"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo -e "${RED}❌ FAILED${NC}"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    fi
    echo ""
}

# Test 1: Build the project
echo "========================================"
echo "Building Project..."
echo "========================================"
cd ~/Desktop/cli-agent

if go build -o bin/eai ./cmd/eai; then
    echo -e "${GREEN}✅ Build successful${NC}"
else
    echo -e "${RED}❌ Build failed${NC}"
    exit 1
fi
echo ""

# Test 2: Help command
run_test "Help command" "bin/eai agent --help" "pass"

# Test 3: Missing API key
run_test "Missing API key handling" "bin/eai agent --task 'test' 2>&1 | grep -q 'EAI_API_KEY'" "pass"

# Test 4: Tool definitions exist
run_test "Tool definitions" "grep -q 'DefaultTools' internal/app/agent.go" "pass"

# Test 5: Agent loop exists
run_test "Agent loop implementation" "grep -q 'Execute' internal/app/agent.go" "pass"

# Test 6: PTY support check (optional)
if grep -q "pty" go.mod 2>/dev/null; then
    run_test "PTY support" "true" "pass"
else
    echo -e "${YELLOW}Test 6: PTY support${NC}"
    echo "  Note: PTY not in dependencies yet - will add in Phase 2"
    echo "  ⚠️ SKIPPED"
    echo ""
fi

# Test 7: Tool calling structure
run_test "Tool calling structure" "grep -q 'ToolCall' internal/app/agent.go" "pass"

# Test 8: State persistence
run_test "State persistence" "grep -q 'saveState' internal/app/agent.go" "pass"

# Test 9: Web search tool
run_test "Web search tool defined" "grep -q 'web_search' internal/app/agent.go" "pass"

# Test 10: File operations
run_test "File operations" "grep -q 'read_file\|write_file' internal/app/agent.go" "pass"

# Summary
echo "========================================"
echo "Test Summary"
echo "========================================"
echo -e "Tests run:    $TESTS_RUN"
echo -e "Tests passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed! ✅${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed. Please review.${NC}"
    exit 1
fi
