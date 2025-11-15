#!/bin/bash

# Deployment script for Fractals v1
# Run this script from the project directory (e.g., /var/www/html/donsmoore.com/fractals/v1)

set -e  # Exit on error

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

PROJECT_DIR="$SCRIPT_DIR"
echo "🚀 Starting Fractals v1 deployment..."
echo "📁 Project directory: $PROJECT_DIR"

# Fix Git ownership and permissions (if needed)
echo "🔧 Fixing Git permissions..."
CURRENT_USER=$(whoami)
# Fix ownership of entire directory temporarily for git operations
sudo chown -R $CURRENT_USER:$CURRENT_USER .
if [ -d .git ]; then
    git config --global --add safe.directory "$PROJECT_DIR"
fi

# Pull latest changes from GitHub (if using git)
if [ -d .git ]; then
    echo "📥 Pulling latest changes from GitHub..."
    # Try to detect the current branch, default to main
    BRANCH=$(git branch --show-current 2>/dev/null || echo "main")
    git pull origin $BRANCH || echo "⚠️  Git pull failed, continuing with local code..."
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

