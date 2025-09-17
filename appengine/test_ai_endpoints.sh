#!/bin/bash

# Test script for new AI endpoints
# Tests both legacy and new provider-agnostic endpoints

set -e

HOST="localhost:8080"
echo "Testing AI endpoints on $HOST"
echo "================================"

# Function to make HTTP requests with proper error handling
make_request() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    local description="$4"

    echo
    echo "Testing: $description"
    echo "Endpoint: $method $endpoint"

    if [[ "$method" == "GET" ]]; then
        response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "http://$HOST$endpoint" || echo "CURL_ERROR")
    else
        response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X "$method" -H "Content-Type: application/json" -d "$data" "http://$HOST$endpoint" || echo "CURL_ERROR")
    fi

    if [[ "$response" == "CURL_ERROR" ]]; then
        echo "❌ CURL Error - Server may not be running"
        return 1
    fi

    http_status=$(echo "$response" | grep "HTTP_STATUS:" | cut -d: -f2)
    body=$(echo "$response" | grep -v "HTTP_STATUS:")

    echo "Status: $http_status"
    echo "Response: $body"

    if [[ "$http_status" =~ ^2[0-9][0-9]$ ]]; then
        echo "✅ SUCCESS"
    elif [[ "$http_status" == "503" ]]; then
        echo "⚠️  Service Unavailable (AI service may not be initialized)"
    else
        echo "❌ HTTP Error: $http_status"
    fi
}

echo "Testing AI Provider Status endpoint..."
make_request "GET" "/api/ai/provider/status" "" "Get current AI provider status"

echo
echo "Testing Legacy Claude endpoint..."
make_request "POST" "/claude" '{"message": "Hello, this is a test message. Please respond briefly."}' "Legacy Claude API endpoint"

echo
echo "Testing new Provider-agnostic endpoint..."
make_request "POST" "/api/ai/v2/chat" '{"message": "Hello, this is a test of the new AI API. Please respond briefly.", "session_id": "test-session"}' "New provider-agnostic chat endpoint"

echo
echo "Testing Provider Switch endpoint (should fail without valid API key)..."
make_request "POST" "/api/ai/provider/switch" '{"provider": "openai", "session_id": "test"}' "Provider switch endpoint (no API key)"

echo
echo "================================"
echo "AI Endpoints Test Complete"
echo
echo "Note: Some tests may fail if:"
echo "  - Server is not running (start with: go run .)"
echo "  - AI service is not properly configured"
echo "  - Claude API key is invalid or missing"
echo "  - OpenAI provider is not implemented yet"