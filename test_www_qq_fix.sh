#!/bin/bash

# Build the binary
echo "Building rec53..."
go build -o rec53 ./cmd
if [ $? -ne 0 ]; then
    echo "Build failed"
    exit 1
fi

# Start the server in background
echo "Starting rec53 server..."
./rec53 -listen 127.0.0.1:53 -log-level debug > /tmp/rec53_test.log 2>&1 &
REC53_PID=$!

# Wait for server to start
sleep 2

# Test www.qq.com query
echo "Testing www.qq.com query..."
RESULT=$(timeout 10 dig @127.0.0.1 www.qq.com +short 2>&1)

# Kill the server
kill $REC53_PID 2>/dev/null
wait $REC53_PID 2>/dev/null

# Check if we got a result
if echo "$RESULT" | grep -q "CNAME\|183.194"; then
    echo "✓ SUCCESS: Got correct result"
    echo "Result:"
    echo "$RESULT"
    exit 0
else
    echo "✗ FAILED: No valid result"
    echo "Result:"
    echo "$RESULT"
    echo ""
    echo "Last 50 lines of log:"
    tail -50 /tmp/rec53_test.log
    exit 1
fi
