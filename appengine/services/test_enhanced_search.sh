#!/bin/bash

echo "🧪 Running Enhanced SearchDocuments QA Test Suite"
echo "================================================"

cd /Users/ericmaibach/Documents/repos/Arcadia/appengine/services

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

test_count=0
pass_count=0
fail_count=0

run_test() {
    local test_name="$1"
    local description="$2"

    echo -e "\n${YELLOW}Testing: $description${NC}"
    echo "Command: go test -run \"^$test_name$\" -v"

    test_count=$((test_count + 1))

    if go test -run "^$test_name$" -v; then
        echo -e "${GREEN}✅ PASS: $test_name${NC}"
        pass_count=$((pass_count + 1))
    else
        echo -e "${RED}❌ FAIL: $test_name${NC}"
        fail_count=$((fail_count + 1))
    fi
}

echo -e "\n📋 Test Plan: Enhanced SearchDocuments Implementation"
echo "======================================================"

# Core enhanced functionality tests
run_test "TestSearchConfig" "SearchConfig structure and defaults"
run_test "TestEnhancedDocumentSearchResult" "Enhanced result structure serialization"
run_test "TestTruncateContent" "Content truncation functionality"
run_test "TestExtractContentPreview" "Content preview extraction"
run_test "TestExtractContextHighlights" "Context highlight extraction"

# Integration tests
run_test "TestEmbeddingService_SearchDocuments" "Original SearchDocuments method"
run_test "TestEmbeddingService_SearchDocumentsEnhanced" "Enhanced SearchDocumentsEnhanced method"
run_test "TestSearchDocumentsEnhanced_EdgeCases" "Edge cases and error handling"

# Claude integration tests
run_test "TestClaudeServiceIntegration_SearchDocuments" "Claude service integration"

# Performance tests
run_test "TestPerformanceComparison" "Performance comparison between methods"

# Additional comprehensive tests
echo -e "\n${YELLOW}Running additional validation tests...${NC}"

# Test that all existing embedding tests still pass
echo -e "\n${YELLOW}Validating backward compatibility...${NC}"
if go test -run "TestEmbedding" -v; then
    echo -e "${GREEN}✅ PASS: All embedding tests${NC}"
    pass_count=$((pass_count + 1))
else
    echo -e "${RED}❌ FAIL: Some embedding tests failed${NC}"
    fail_count=$((fail_count + 1))
fi
test_count=$((test_count + 1))

# Test that chunking and other core functionality still works
echo -e "\n${YELLOW}Validating core functionality...${NC}"
if go test -run "TestSimpleTextChunker\|TestCosineSimilarity\|TestFileExtensionDetection\|TestContentTypeDetection" -v; then
    echo -e "${GREEN}✅ PASS: Core functionality tests${NC}"
    pass_count=$((pass_count + 1))
else
    echo -e "${RED}❌ FAIL: Some core functionality tests failed${NC}"
    fail_count=$((fail_count + 1))
fi
test_count=$((test_count + 1))

# Final summary
echo -e "\n🏁 Test Results Summary"
echo "======================"
echo -e "Total Tests: $test_count"
echo -e "${GREEN}Passed: $pass_count${NC}"
echo -e "${RED}Failed: $fail_count${NC}"

if [ $fail_count -eq 0 ]; then
    echo -e "\n${GREEN}🎉 All tests passed! Enhanced SearchDocuments implementation is ready.${NC}"
    exit 0
else
    echo -e "\n${RED}💥 $fail_count test(s) failed. Please review the failures above.${NC}"
    exit 1
fi