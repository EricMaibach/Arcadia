#!/bin/bash

# Test runner script for Arcadia App Engine

set -e

echo "🧪 Running Arcadia App Engine Tests"
echo "=================================="

# Function to print section headers
print_section() {
    echo
    echo "📋 $1"
    echo "$(printf '%.0s-' {1..40})"
}

# Run basic tests
print_section "Unit Tests"
go test -v

# Run tests with coverage
print_section "Coverage Report"
go test -cover -coverprofile=coverage.out
if [ -f coverage.out ]; then
    go tool cover -html=coverage.out -o coverage.html
    echo "📊 Coverage report generated: coverage.html"
fi

# Run benchmarks
print_section "Benchmark Tests"
go test -bench=. -benchmem

# Run race condition tests
print_section "Race Condition Tests"
go test -race

# Lint and format checks
print_section "Code Quality"
if command -v golint &> /dev/null; then
    echo "🔍 Running golint..."
    golint ./...
else
    echo "⚠️  golint not installed, skipping..."
fi

if command -v gofmt &> /dev/null; then
    echo "🔍 Checking gofmt..."
    if [ -n "$(gofmt -l .)" ]; then
        echo "❌ Code needs formatting. Run: gofmt -w ."
        gofmt -l .
    else
        echo "✅ Code is properly formatted"
    fi
fi

# Check for common issues
print_section "Static Analysis"
if command -v go &> /dev/null; then
    echo "🔍 Running go vet..."
    go vet ./...
fi

echo
echo "🎉 All tests completed!"
if [ -f coverage.html ]; then
    echo "📊 Open coverage.html in your browser to view detailed coverage report"
fi