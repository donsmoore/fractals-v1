#!/bin/bash

# Script to help fix SSL certificate path issues in Apache

echo "🔍 Checking SSL certificate locations..."

# Check common certificate locations
echo "Checking /etc/letsencrypt/live/donsmoore.com/..."
if [ -d "/etc/letsencrypt/live/donsmoore.com" ]; then
    echo "✅ Certificate directory exists"
    ls -la /etc/letsencrypt/live/donsmoore.com/
else
    echo "❌ Certificate directory not found at /etc/letsencrypt/live/donsmoore.com/"
fi

# Check for other certificate locations
echo ""
echo "Checking for other certificate files..."
find /etc/letsencrypt/live -name "fullchain.pem" 2>/dev/null | head -5

# Check Apache config
echo ""
echo "Current SSL certificate configuration:"
grep -n "SSLCertificateFile" /etc/apache2/sites-enabled/donsmoore.com-le-ssl.conf 2>/dev/null || echo "Config file not found or no SSLCertificateFile found"

echo ""
echo "To fix this, you can:"
echo "1. If certificate exists elsewhere, update the path in the config"
echo "2. If certificate doesn't exist, generate it with: sudo certbot --apache -d donsmoore.com -d www.donsmoore.com"
echo "3. Or temporarily comment out SSL lines if testing HTTP first"

