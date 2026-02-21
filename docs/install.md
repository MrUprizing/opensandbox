# Installation

### Prerequisites

- Docker
- Go
- gVisor (Production & unsafe environments)

## Update

```bash
sudo apt update
sudo apt upgrade -y
```

## Installation Steps

### Go

Download Go

```bash
wget https://dl.google.com/go/go1.26.0.linux-amd64.tar.gz
```

Install Go

```bash
rm -rf /usr/local/go
tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz
echo 'export PATH=/usr/local/go/bin:$PATH' >> /root/.bashrc
bash
```

Verify Installation

```bash
go version
```

### Docker

Install Docker

```bash
sudo apt install -y docker.io docker-compose
```

Start and enable Docker

```bash
sudo systemctl start docker
sudo systemctl enable docker
```

Verify Installation

```bash
docker --version
```

### gVisor (Production & unsafe environments)

```bash
#!/bin/bash
set -e

# 1. Add gVisor repository
curl -fsSL https://gvisor.dev/archive.key | sudo gpg --dearmor -o /usr/share/keyrings/gvisor-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] https://storage.googleapis.com/gvisor/releases release main" | sudo tee /etc/apt/sources.list.d/gvisor.list > /dev/null

# 2. Install runsc
sudo apt-get update && sudo apt-get install -y runsc

# 3. Set as default Docker runtime
RUNSC_PATH=$(which runsc)
sudo mkdir -p /etc/docker
cat <<EOF | sudo tee /etc/docker/daemon.json
{
  "default-runtime": "runsc",
  "runtimes": {
    "runsc": {
      "path": "${RUNSC_PATH}"
    }
  }
}
EOF

# 4. Restart Docker
sudo systemctl restart docker

# 5. Verify
echo "--- Default Runtime ---"
docker info 2>/dev/null | grep "Default Runtime"
echo "--- Test ---"
docker run --rm hello-world
```

Recomended:

```bash
sudo reboot
```

# Opensandbox

Install and run Opensandbox

```bash
git clone https://github.com/MrUprizing/opensandbox.git 
cd opensandbox
go run cmd/api/main.go
```
