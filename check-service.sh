#!/bin/bash

# Script to diagnose and fix the 203/EXEC error

echo "🔍 Diagnosing fractals-v1.service issue..."
echo ""

# Check if binary exists
BINARY_PATH="/var/www/html/donsmoore.com/fractals/v1/fractals"
echo "1. Checking if binary exists at: $BINARY_PATH"
if [ -f "$BINARY_PATH" ]; then
    echo "   ✅ Binary exists"
    ls -la "$BINARY_PATH"
else
    echo "   ❌ Binary does NOT exist!"
    echo "   Need to rebuild it."
fi

echo ""
echo "2. Checking service file configuration:"
grep "ExecStart" /etc/systemd/system/fractals-v1.service

echo ""
echo "3. Checking if Go is available:"
if command -v go &> /dev/null; then
    echo "   ✅ Go is installed: $(go version)"
else
    echo "   ❌ Go is NOT installed or not in PATH"
    echo "   Run: source ~/.bashrc or install Go"
fi

echo ""
echo "4. Checking working directory:"
grep "WorkingDirectory" /etc/systemd/system/fractals-v1.service

echo ""
echo "📋 To fix:"
echo "   cd /var/www/html/donsmoore.com/fractals/v1"
echo "   go build -o fractals main.go"
echo "   chmod +x fractals"
echo "   sudo chown www-data:www-data fractals"
echo "   sudo systemctl restart fractals-v1.service"

