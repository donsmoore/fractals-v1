# Fixing SSL Certificate Path Error

If you're getting this error:
```
SSLCertificateFile: file '/etc/letsencrypt/live/donsmoore.com/fullchain.pem' does not exist or is empty
```

## Option 1: Check if Certificate Exists Elsewhere

Run this to find your certificate:
```bash
# Check if certificate directory exists
ls -la /etc/letsencrypt/live/

# Find all certificate files
find /etc/letsencrypt/live -name "fullchain.pem" 2>/dev/null
```

## Option 2: Generate SSL Certificate

If the certificate doesn't exist, generate it:
```bash
sudo certbot --apache -d donsmoore.com -d www.donsmoore.com
```

This will:
- Generate the SSL certificate
- Automatically configure Apache
- Set up auto-renewal

## Option 3: Temporarily Test with HTTP (Port 80)

If you want to test the fractals app first without SSL, you can:

1. **Add the ProxyPass to the HTTP VirtualHost (port 80) instead:**

Edit the HTTP VirtualHost:
```bash
sudo nano /etc/apache2/sites-enabled/donsmoore.com.conf
# or
sudo nano /etc/apache2/sites-available/donsmoore.com.conf
```

Add the ProxyPass configuration to the `<VirtualHost *:80>` block:
```apache
<VirtualHost *:80>
    ServerName donsmoore.com
    ServerAlias www.donsmoore.com
    
    # ... existing config ...
    
    # Fractals v1 reverse proxy
    ProxyPreserveHost On
    ProxyPass /fractals/v1/ http://localhost:8080/
    ProxyPassReverse /fractals/v1/ http://localhost:8080/
    
    <Location /fractals/v1>
        Require all granted
    </Location>
</VirtualHost>
```

2. **Or fix the SSL config by commenting out SSL lines temporarily:**

Edit the SSL config:
```bash
sudo nano /etc/apache2/sites-enabled/donsmoore.com-le-ssl.conf
```

Comment out the SSL certificate lines (add # at the beginning):
```apache
# SSLEngine on
# SSLCertificateFile /etc/letsencrypt/live/donsmoore.com/fullchain.pem
# SSLCertificateKeyFile /etc/letsencrypt/live/donsmoore.com/privkey.pem
```

**Note:** This will disable SSL for that VirtualHost. Only do this for testing.

## Option 4: Check Certificate Path in Config

The certificate path might be wrong. Check what's actually in the config:
```bash
grep -A 3 "SSLCertificateFile" /etc/apache2/sites-enabled/donsmoore.com-le-ssl.conf
```

If the path is wrong, update it to the correct location.

## Recommended Solution

The best approach is to generate the SSL certificate properly:

```bash
# Install certbot if not installed
sudo apt update
sudo apt install certbot python3-certbot-apache

# Generate certificate
sudo certbot --apache -d donsmoore.com -d www.donsmoore.com
```

This will automatically:
- Generate the certificate
- Configure Apache correctly
- Set up auto-renewal
- Add the certificate paths to your VirtualHost

After running certbot, you can then add the Fractals v1 ProxyPass configuration to the SSL VirtualHost.

