# Production Apache Configuration for Fractals v1

This guide will help you add the Fractals v1 reverse proxy configuration to your existing `donsmoore.com` VirtualHost.

## Step 1: Find Your Apache Configuration

Your Apache configuration for `donsmoore.com` is likely in one of these files:
- `/etc/apache2/sites-available/donsmoore-timeclock-v3.conf`
- `/etc/apache2/sites-available/000-default.conf` (if using default)
- `/etc/apache2/sites-available/default-ssl.conf` (for HTTPS)

Check which file contains your `donsmoore.com` VirtualHost:
```bash
grep -r "ServerName donsmoore.com" /etc/apache2/sites-available/
```

## Step 2: Edit the SSL VirtualHost (HTTPS)

Since you're accessing via `https://donsmoore.com/fractals/v1/`, you need to add the configuration to the HTTPS VirtualHost (port 443).

```bash
sudo nano /etc/apache2/sites-available/[your-config-file].conf
```

## Step 3: Add the ProxyPass Configuration

Inside your `<VirtualHost *:443>` block, add these lines **before** the closing `</VirtualHost>` tag:

```apache
<VirtualHost *:443>
    ServerName donsmoore.com
    ServerAlias www.donsmoore.com
    
    # ... existing configuration ...
    
    # Fractals v1 reverse proxy
    ProxyPreserveHost On
    ProxyPass /fractals/v1/ http://localhost:8080/
    ProxyPassReverse /fractals/v1/ http://localhost:8080/
    
    <Location /fractals/v1>
        Require all granted
    </Location>
    
    # ... rest of your configuration ...
</VirtualHost>
```

**Important:** Notice the trailing slashes in `/fractals/v1/` and `http://localhost:8080/` - these are required to prevent redirects!

## Step 4: Enable Proxy Modules (if not already enabled)

```bash
sudo a2enmod proxy proxy_http
```

## Step 5: Test and Restart Apache

```bash
# Test the configuration
sudo apache2ctl configtest

# If test passes, restart Apache
sudo systemctl restart apache2
```

## Step 6: Verify the Service is Running

Make sure the Fractals v1 service is running:

```bash
sudo systemctl status fractals-v1.service
```

If it's not running:
```bash
sudo systemctl start fractals-v1.service
sudo systemctl enable fractals-v1.service
```

## Step 7: Test the Application

```bash
# Test the API directly
curl http://localhost:8080/api/health

# Test through Apache proxy
curl https://donsmoore.com/fractals/v1/api/health
```

## Troubleshooting

### Still Getting "Forbidden" Error

1. **Check Apache error logs:**
   ```bash
   sudo tail -f /var/log/apache2/error.log
   ```

2. **Verify the Location directive:**
   Make sure you have:
   ```apache
   <Location /fractals/v1>
       Require all granted
   </Location>
   ```

3. **Check if proxy modules are enabled:**
   ```bash
   apache2ctl -M | grep proxy
   ```
   Should show: `proxy_module` and `proxy_http_module`

4. **Verify the Go service is running:**
   ```bash
   sudo systemctl status fractals-v1.service
   curl http://localhost:8080/api/health
   ```

### Getting 404 Instead of Forbidden

If you get 404, the proxy might not be matching. Check:
- The ProxyPass path has trailing slashes: `/fractals/v1/`
- The target URL has trailing slash: `http://localhost:8080/`
- The Location path matches: `/fractals/v1`

### Getting Redirects

If requests are being redirected, make sure:
- Both ProxyPass paths have trailing slashes
- The Location directive uses `/fractals/v1` (no trailing slash)

## Example Complete VirtualHost

Here's an example of what your VirtualHost might look like with both timeclock and fractals:

```apache
<VirtualHost *:443>
    ServerName donsmoore.com
    ServerAlias www.donsmoore.com
    
    DocumentRoot /var/www/html
    
    # Timeclock v3
    Alias /timeclock/v3 /var/www/html/donsmoore.com/timeclock/v3/public
    <Directory /var/www/html/donsmoore.com/timeclock/v3/public>
        Options -Indexes +FollowSymLinks
        AllowOverride All
        Require all granted
        DirectoryIndex index.php index.html
        RewriteEngine On
        RewriteCond %{REQUEST_FILENAME} !-f
        RewriteCond %{REQUEST_FILENAME} !-d
        RewriteRule ^ index.php [L]
    </Directory>
    
    # Fractals v1 reverse proxy
    ProxyPreserveHost On
    ProxyPass /fractals/v1/ http://localhost:8080/
    ProxyPassReverse /fractals/v1/ http://localhost:8080/
    
    <Location /fractals/v1>
        Require all granted
    </Location>
    
    # SSL Configuration
    SSLEngine on
    SSLCertificateFile /etc/letsencrypt/live/donsmoore.com/fullchain.pem
    SSLCertificateKeyFile /etc/letsencrypt/live/donsmoore.com/privkey.pem
    
    ErrorLog ${APACHE_LOG_DIR}/donsmoore-error.log
    CustomLog ${APACHE_LOG_DIR}/donsmoore-access.log combined
</VirtualHost>
```

