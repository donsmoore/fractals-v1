# Step-by-Step Go Installation Guide for Ubuntu 24.04

Follow these steps to install Go (Golang) on your Ubuntu 24.04 system:

## Step 1: Download Go
The Go archive has already been downloaded to `/tmp/go1.23.4.linux-amd64.tar.gz`

If you need to download it again, run:
```bash
cd /tmp
wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
```

## Step 2: Remove any existing Go installation (if present)
```bash
sudo rm -rf /usr/local/go
```

## Step 3: Extract Go to /usr/local
```bash
sudo tar -C /usr/local -xzf /tmp/go1.23.4.linux-amd64.tar.gz
```

This will extract Go to `/usr/local/go`.

## Step 4: Add Go to your PATH
You need to add Go's bin directory to your PATH. Choose one of these methods:

### Option A: Add to your current session (temporary)
```bash
export PATH=$PATH:/usr/local/go/bin
```

### Option B: Add permanently to your shell profile (recommended)
For bash (default shell):
```bash
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

For zsh:
```bash
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.zshrc
source ~/.zshrc
```

## Step 5: Verify the installation
```bash
go version
```

You should see output like: `go version go1.23.4 linux/amd64`

## Step 6: Verify Go environment
```bash
go env
```

This will show your Go environment variables. The important ones are:
- `GOROOT`: Should be `/usr/local/go`
- `GOPATH`: Usually `~/go` (will be created when you first use Go)

## Step 7: Test with your fractals project
Navigate to your project and test:
```bash
cd /var/www/html/fractals/v1
go version
go mod tidy
go build -o fractals main.go
```

## Troubleshooting

If `go version` doesn't work after adding to PATH:
1. Make sure you've run `source ~/.bashrc` (or `source ~/.zshrc`)
2. Check that `/usr/local/go/bin` exists: `ls -la /usr/local/go/bin`
3. Verify PATH: `echo $PATH` (should include `/usr/local/go/bin`)
4. Try opening a new terminal window

## Clean up
After installation is complete, you can remove the downloaded archive:
```bash
rm /tmp/go1.23.4.linux-amd64.tar.gz
```

