#!/bin/bash

# Script to fix the Apache redirect issue for Fractals v1
# The ProxyPass configuration needs to be more specific

set -e

echo "🔧 Fixing Apache ProxyPass configuration to prevent redirects..."

SSL_CONF="/etc/apache2/sites-available/default-ssl.conf"

if [ ! -f "$SSL_CONF" ]; then
    echo "❌ SSL config not found: $SSL_CONF"
    exit 1
fi

# Check if the fix is already applied
if grep -q "ProxyPass /fractals/v1/ http://localhost:8080/" "$SSL_CONF"; then
    echo "ℹ️  Configuration already has trailing slash fix"
else
    echo "📝 Updating ProxyPass configuration..."
    
    # Create backup
    sudo cp "$SSL_CONF" "${SSL_CONF}.bak.$(date +%Y%m%d_%H%M%S)"
    
    # Replace the ProxyPass lines to include trailing slash
    sudo sed -i 's|ProxyPass /fractals/v1 http://localhost:8080/|ProxyPass /fractals/v1/ http://localhost:8080/|g' "$SSL_CONF"
    sudo sed -i 's|ProxyPassReverse /fractals/v1 http://localhost:8080/|ProxyPassReverse /fractals/v1/ http://localhost:8080/|g' "$SSL_CONF"
    
    echo "✅ Updated ProxyPass configuration"
fi

# Test Apache configuration
echo "🧪 Testing Apache configuration..."
if sudo apache2ctl configtest; then
    echo "✅ Apache configuration is valid"
    echo "🔄 Restarting Apache..."
    sudo systemctl restart apache2
    echo "✅ Apache restarted"
    echo ""
    echo "🌐 The redirect issue should now be fixed!"
else
    echo "❌ Apache configuration test failed!"
    echo "Please check the configuration manually"
    exit 1
fi

