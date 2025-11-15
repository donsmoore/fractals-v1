#!/bin/bash

# Deployment script for Fractals v1
# Run this script on your AWS EC2 server in /var/www/html/fractals/v1

set -e  # Exit on error

echo "🚀 Starting Fractals v1 deployment..."

# Navigate to application directory
cd /var/www/html/fractals/v1

# Fix Git ownership and permissions (if needed)
echo "🔧 Fixing Git permissions..."
CURRENT_USER=$(whoami)
# Fix ownership of entire directory temporarily for git operations
sudo chown -R $CURRENT_USER:$CURRENT_USER .
if [ -d .git ]; then
    git config --global --add safe.directory /var/www/html/fractals/v1
fi

# Pull latest changes from GitHub (if using git)
if [ -d .git ]; then
    echo "📥 Pulling latest changes from GitHub..."
    git pull origin main || echo "⚠️  Git pull failed or not a git repo, continuing..."
fi

# Build the Go application
echo "🔨 Building Go application..."
go build -o fractals main.go

# Set proper permissions
echo "🔐 Setting permissions..."
sudo chown www-data:www-data fractals
sudo chmod +x fractals

# Install/update systemd service
echo "⚙️  Installing systemd service..."
sudo cp fractals-v1.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable fractals-v1.service
sudo systemctl restart fractals-v1.service

# Check service status
echo "📊 Checking service status..."
sudo systemctl status fractals-v1.service --no-pager -l || true

# Configure Apache (if not already configured)
echo "🌐 Configuring Apache..."
if [ ! -f /etc/apache2/sites-enabled/fractals-v1.conf ]; then
    echo "⚠️  Apache configuration not found. Please:"
    echo "   1. Add the reverse proxy configuration from apache-config.conf to your Apache VirtualHost"
    echo "   2. Enable required modules: sudo a2enmod proxy proxy_http"
    echo "   3. Restart Apache: sudo systemctl restart apache2"
else
    echo "✅ Apache configuration already exists"
    # Reload Apache to pick up changes
    sudo systemctl reload apache2 || sudo systemctl restart apache2
fi

# Ensure .git remains owned by current user for future pulls
if [ -d .git ]; then
    sudo chown -R $CURRENT_USER:$CURRENT_USER .git
fi

echo "✅ Deployment complete!"
echo "🌐 Access your app at: https://localhost/fractals/v1"
echo "📊 Check service logs with: sudo journalctl -u fractals-v1 -f"

