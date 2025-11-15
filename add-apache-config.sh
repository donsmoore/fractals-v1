#!/bin/bash

# Script to add Fractals v1 reverse proxy configuration to Apache
# This will add the configuration to both HTTP and HTTPS VirtualHosts

set -e

echo "🔧 Adding Fractals v1 reverse proxy configuration to Apache..."

# Function to add configuration to a file
add_proxy_config() {
    local conf_file="$1"
    local vhost_name="$2"
    
    if [ ! -f "$conf_file" ]; then
        echo "⚠️  $vhost_name config not found: $conf_file"
        return 1
    fi
    
    if grep -q "fractals/v1" "$conf_file"; then
        echo "ℹ️  $vhost_name already has fractals configuration"
        return 0
    fi
    
    echo "📝 Adding configuration to $vhost_name..."
    
    # Create a temporary file with the configuration
    local temp_file=$(mktemp)
    
    # Copy everything before the closing VirtualHost tag
    awk '/^<\/VirtualHost>/ { 
        print "\t# Fractals v1 reverse proxy"
        print "\tProxyPreserveHost On"
        print "\tProxyPass /fractals/v1 http://localhost:8080/"
        print "\tProxyPassReverse /fractals/v1 http://localhost:8080/"
        print ""
        print "\t<Location /fractals/v1>"
        print "\t\tRequire all granted"
        print "\t</Location>"
        print ""
    }1' "$conf_file" > "$temp_file"
    
    # Backup original and replace
    sudo cp "$conf_file" "${conf_file}.bak"
    sudo cp "$temp_file" "$conf_file"
    rm "$temp_file"
    
    echo "✅ Added to $vhost_name"
}

# Add to HTTP VirtualHost (000-default.conf)
add_proxy_config "/etc/apache2/sites-available/000-default.conf" "HTTP VirtualHost"

# Add to HTTPS VirtualHost (default-ssl.conf)
add_proxy_config "/etc/apache2/sites-available/default-ssl.conf" "HTTPS VirtualHost"

# Test Apache configuration
echo "🧪 Testing Apache configuration..."
if sudo apache2ctl configtest; then
    echo "✅ Apache configuration is valid"
    echo "🔄 Restarting Apache..."
    sudo systemctl restart apache2
    echo "✅ Apache restarted"
    echo ""
    echo "🌐 Your fractals app should now be accessible at:"
    echo "   - http://localhost/fractals/v1"
    echo "   - https://localhost/fractals/v1"
else
    echo "❌ Apache configuration test failed!"
    echo "Please check the configuration manually"
    exit 1
fi

