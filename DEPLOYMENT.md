# Fractals v1 Deployment Guide

This guide explains how to deploy the Fractals v1 Go application to your server (including AWS EC2).

## Prerequisites

- Go 1.21+ installed
- Apache web server installed
- Systemd (standard on Ubuntu/Debian)

## Local Setup (for testing)

### Step 1: Build the application
```bash
cd /var/www/html/fractals/v1
go build -o fractals main.go
```

### Step 2: Test run locally
```bash
./fractals
# Or with custom port:
PORT=8080 ./fractals
```

Access at: http://localhost:8080

## Production Setup

### Step 1: Build the application
```bash
cd /var/www/html/fractals/v1
go build -o fractals main.go
sudo chown www-data:www-data fractals
sudo chmod +x fractals
```

### Step 2: Install systemd service
```bash
sudo cp fractals-v1.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable fractals-v1.service
sudo systemctl start fractals-v1.service
```

Check service status:
```bash
sudo systemctl status fractals-v1.service
```

View logs:
```bash
sudo journalctl -u fractals-v1 -f
```

### Step 3: Configure Apache reverse proxy

#### Option A: Add to existing VirtualHost

Edit your existing Apache VirtualHost configuration (usually in `/etc/apache2/sites-available/000-default.conf` or your custom site config):

```apache
<VirtualHost *:80>
    # ... existing configuration ...
    
    # Enable proxy modules (run once)
    # sudo a2enmod proxy proxy_http
    
    # Reverse proxy for Fractals v1
    ProxyPreserveHost On
    ProxyPass /fractals/v1 http://localhost:8080/
    ProxyPassReverse /fractals/v1 http://localhost:8080/
    
    <Location /fractals/v1>
        Require all granted
    </Location>
</VirtualHost>
```

#### Option B: Create separate site configuration

1. Copy the configuration:
```bash
sudo cp apache-config.conf /etc/apache2/sites-available/fractals-v1.conf
```

2. Edit the file to match your setup, then enable:
```bash
sudo a2ensite fractals-v1.conf
```

3. Enable required Apache modules:
```bash
sudo a2enmod proxy proxy_http rewrite headers
```

4. Test Apache configuration:
```bash
sudo apache2ctl configtest
```

5. Restart Apache:
```bash
sudo systemctl restart apache2
```

### Step 4: Verify deployment

1. Check the Go service is running:
```bash
sudo systemctl status fractals-v1.service
curl http://localhost:8080/api/health
```

2. Check Apache can proxy:
```bash
curl http://localhost/fractals/v1/api/health
```

3. Access in browser:
- Local: http://localhost/fractals/v1
- Production: https://yourdomain.com/fractals/v1

## Using the Deployment Script

For automated deployment (especially useful for AWS EC2):

```bash
cd /var/www/html/fractals/v1
chmod +x deploy.sh
./deploy.sh
```

The script will:
- Pull latest code (if using git)
- Build the Go application
- Install/restart the systemd service
- Configure Apache (with instructions if needed)

## AWS EC2 Deployment

### Initial Setup

1. SSH into your EC2 instance
2. Install Go (if not already installed) - see `INSTALL_GO.md`
3. Clone or upload your code to `/var/www/html/fractals/v1`
4. Run the deployment script:
```bash
cd /var/www/html/fractals/v1
./deploy.sh
```

### Updating the Application

1. SSH into EC2
2. Navigate to project directory
3. Pull latest changes (if using git) or upload new files
4. Run deployment script:
```bash
cd /var/www/html/fractals/v1
./deploy.sh
```

Or manually:
```bash
cd /var/www/html/fractals/v1
go build -o fractals main.go
sudo systemctl restart fractals-v1.service
```

## Troubleshooting

### Service won't start
```bash
# Check logs
sudo journalctl -u fractals-v1 -n 50

# Check if port is in use
sudo netstat -tlnp | grep 8080

# Test the binary manually
cd /var/www/html/fractals/v1
sudo -u www-data ./fractals
```

### Apache proxy not working
```bash
# Check Apache error logs
sudo tail -f /var/log/apache2/error.log

# Verify modules are enabled
apache2ctl -M | grep proxy

# Test Apache configuration
sudo apache2ctl configtest
```

### Permission issues
```bash
# Fix ownership
sudo chown -R www-data:www-data /var/www/html/fractals/v1
sudo chmod +x /var/www/html/fractals/v1/fractals
```

## Service Management

```bash
# Start service
sudo systemctl start fractals-v1.service

# Stop service
sudo systemctl stop fractals-v1.service

# Restart service
sudo systemctl restart fractals-v1.service

# View status
sudo systemctl status fractals-v1.service

# View logs
sudo journalctl -u fractals-v1 -f
```

## Environment Variables

The application supports the following environment variables:

- `PORT`: Port to listen on (default: 8080)

Set in the systemd service file or export before running:
```bash
export PORT=8080
./fractals
```

