# Fixing 503 Service Unavailable Error

The error `status=203/EXEC` means the executable file cannot be executed.

## Quick Fix Steps

Run these commands on your production server:

```bash
cd /var/www/html/donsmoore.com/fractals/v1

# 1. Check if the binary exists
ls -la fractals

# 2. If it doesn't exist or is old, rebuild it
go build -o fractals main.go

# 3. Make sure it has execute permissions
chmod +x fractals

# 4. Verify the binary works
./fractals &
# Test it: curl http://localhost:8080/api/health
# Then kill it: pkill fractals

# 5. Set proper ownership
sudo chown www-data:www-data fractals

# 6. Restart the service
sudo systemctl restart fractals-v1.service

# 7. Check status
sudo systemctl status fractals-v1.service
```

## Common Causes

1. **Binary not built** - After code changes, you need to rebuild
2. **Wrong permissions** - Binary needs execute permission
3. **Wrong ownership** - Should be owned by www-data
4. **Path mismatch** - Service file path doesn't match actual location

## Verify Service File Path

Check that the service file has the correct path:
```bash
cat /etc/systemd/system/fractals-v1.service | grep ExecStart
```

Should show: `ExecStart=/var/www/html/donsmoore.com/fractals/v1/fractals`

