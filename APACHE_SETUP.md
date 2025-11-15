# Apache Configuration for Fractals v1

## Quick Fix - Run the script:
```bash
cd /var/www/html/fractals/v1
./add-apache-config.sh
```

## Manual Configuration

If you prefer to add it manually, follow these steps:

### Step 1: Edit the HTTPS VirtualHost (since you're using https://localhost)

```bash
sudo nano /etc/apache2/sites-available/default-ssl.conf
```

### Step 2: Add this configuration BEFORE the closing `</VirtualHost>` tag:

```apache
# Fractals v1 reverse proxy
ProxyPreserveHost On
ProxyPass /fractals/v1 http://localhost:8080/
ProxyPassReverse /fractals/v1 http://localhost:8080/

<Location /fractals/v1>
    Require all granted
</Location>
```

The file should look something like this (add the proxy config before `</VirtualHost>`):

```apache
<VirtualHost *:443>
	ServerAdmin webmaster@localhost
	DocumentRoot /var/www/html
	
	# Alias for the timeclock v3 application
	Alias /timeclock/v3 /var/www/html/timeclock/v3/public
	# ... timeclock config ...

	# Fractals v1 reverse proxy
	ProxyPreserveHost On
	ProxyPass /fractals/v1 http://localhost:8080/
	ProxyPassReverse /fractals/v1 http://localhost:8080/

	<Location /fractals/v1>
		Require all granted
	</Location>

	# SSL Configuration
	SSLEngine on
	# ... rest of SSL config ...
</VirtualHost>
```

### Step 3: Also add to HTTP VirtualHost (for http://localhost)

```bash
sudo nano /etc/apache2/sites-available/000-default.conf
```

Add the same proxy configuration before the closing `</VirtualHost>` tag.

### Step 4: Test and restart Apache

```bash
# Test configuration
sudo apache2ctl configtest

# If test passes, restart Apache
sudo systemctl restart apache2
```

### Step 5: Verify

```bash
# Check the Go service is running
sudo systemctl status fractals-v1.service

# Test the proxy
curl http://localhost/fractals/v1/api/health
curl https://localhost/fractals/v1/api/health
```

Then access in your browser: **https://localhost/fractals/v1**

