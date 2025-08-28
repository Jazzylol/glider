#!/bin/bash

# Glider API Mode Test Script
# This script tests the API endpoints of Glider

API_BASE="http://localhost:9000"

echo "🚀 Testing Glider API Mode..."
echo "=================================="

# Function to make API calls and format output
call_api() {
    local method=$1
    local endpoint=$2
    local description=$3
    
    echo ""
    echo "📡 $description"
    echo "   Method: $method"
    echo "   URL: $API_BASE$endpoint"
    echo "   Response:"
    
    if [ "$method" = "POST" ]; then
        curl -s -X POST "$API_BASE$endpoint" | python3 -m json.tool 2>/dev/null || curl -s -X POST "$API_BASE$endpoint"
    else
        curl -s "$API_BASE$endpoint" | python3 -m json.tool 2>/dev/null || curl -s "$API_BASE$endpoint"
    fi
    
    echo ""
    echo "---"
}

# Test sequence
echo ""
echo "⏰ Waiting 3 seconds for Glider to start..."
sleep 3

# 1. Get initial proxy list
call_api "GET" "/api/proxy/list" "1. Get proxy list"

# 2. Get current proxy
call_api "GET" "/api/proxy/current" "2. Get current proxy"

# 3. Change proxy (first time)
call_api "POST" "/api/proxy/change" "3. Change proxy (attempt 1)"

# 4. Get current proxy after change
call_api "GET" "/api/proxy/current" "4. Get current proxy after change"

# 5. Change proxy (second time)
call_api "POST" "/api/proxy/change" "5. Change proxy (attempt 2)"

# 6. Get current proxy after second change
call_api "GET" "/api/proxy/current" "6. Get current proxy after second change"

# 7. Change proxy (third time)
call_api "POST" "/api/proxy/change" "7. Change proxy (attempt 3)"

echo ""
echo "✅ API testing completed!"
echo ""
echo "📋 Usage Summary:"
echo "   - GET  /api/proxy/list     - Get all available proxies"
echo "   - GET  /api/proxy/current  - Get currently selected proxy"
echo "   - POST /api/proxy/change   - Switch to a different proxy"
echo ""
echo "💡 You can also test manually with:"
echo "   curl http://localhost:9000/api/proxy/current"
echo "   curl -X POST http://localhost:9000/api/proxy/change"
