# GitHub Setup Guide for Fractals v1

This guide will help you push your Fractals v1 project to GitHub and deploy it to AWS EC2.

## Step 1: Create a GitHub Repository

1. Go to [GitHub](https://github.com) and sign in
2. Click the "+" icon in the top right, then "New repository"
3. Name it: `fractals-v1` (or your preferred name)
4. **Do NOT** initialize with README, .gitignore, or license (we already have these)
5. Click "Create repository"

## Step 2: Push to GitHub

Run these commands in your project directory:

```bash
cd /var/www/html/fractals/v1

# Add the remote repository (replace YOUR_USERNAME with your GitHub username)
git remote add origin https://github.com/YOUR_USERNAME/fractals-v1.git

# Rename branch to main if needed (GitHub uses 'main' by default)
git branch -M main

# Push to GitHub
git push -u origin main
```

If you're using SSH instead of HTTPS:
```bash
git remote add origin git@github.com:YOUR_USERNAME/fractals-v1.git
git branch -M main
git push -u origin main
```

## Step 3: Set Up on AWS EC2

### Initial Setup on EC2

1. SSH into your EC2 instance:
```bash
ssh -i your-key.pem ubuntu@your-ec2-ip
```

2. Install Go (if not already installed):
```bash
# Follow the instructions in INSTALL_GO.md
# Or use the quick method:
cd /tmp
wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version
```

3. Clone the repository:
```bash
cd /var/www/html
sudo mkdir -p fractals/v1
sudo chown $USER:$USER fractals/v1
cd fractals/v1
git clone https://github.com/YOUR_USERNAME/fractals-v1.git .
# Or if using SSH:
# git clone git@github.com:YOUR_USERNAME/fractals-v1.git .
```

4. Run the deployment script:
```bash
cd /var/www/html/fractals/v1
chmod +x deploy.sh
./deploy.sh
```

5. Configure Apache (if not already done):
```bash
# Add the reverse proxy configuration to your Apache VirtualHost
# See APACHE_SETUP.md for details
sudo nano /etc/apache2/sites-available/default-ssl.conf
# Add the ProxyPass configuration with trailing slashes:
# ProxyPass /fractals/v1/ http://localhost:8080/
# ProxyPassReverse /fractals/v1/ http://localhost:8080/
sudo systemctl restart apache2
```

## Step 4: Future Updates

To deploy updates from GitHub:

```bash
cd /var/www/html/fractals/v1
./deploy.sh
```

The deployment script will:
- Pull the latest code from GitHub
- Build the Go application
- Restart the systemd service
- Reload Apache

## Troubleshooting

### Git Authentication Issues

If you get authentication errors when pushing/pulling:

**For HTTPS:**
- Use a Personal Access Token instead of password
- Create one at: GitHub → Settings → Developer settings → Personal access tokens
- Use the token as your password when prompted

**For SSH:**
- Generate an SSH key: `ssh-keygen -t ed25519 -C "your_email@example.com"`
- Add the public key to GitHub: Settings → SSH and GPG keys → New SSH key
- Test: `ssh -T git@github.com`

### Permission Issues on EC2

If you get permission errors:
```bash
sudo chown -R $USER:$USER /var/www/html/fractals/v1
sudo chown -R $USER:$USER /var/www/html/fractals/v1/.git
```

### Service Won't Start

Check the logs:
```bash
sudo journalctl -u fractals-v1 -n 50
sudo systemctl status fractals-v1
```

## File Structure

Your repository should contain:
- `main.go` - Main application code
- `go.mod` - Go module definition
- `fractals-v1.service` - Systemd service file
- `deploy.sh` - Deployment script
- `apache-config.conf` - Apache configuration template
- `README.md` - Project documentation
- `.gitignore` - Git ignore rules

The `fractals` binary is built during deployment and is not committed to git.

