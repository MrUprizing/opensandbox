# Installation

### Prerequisites

- Docker
- Go

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

# Opensandbox

Install and run Opensandbox

```bash
git clone https://github.com/MrUprizing/opensandbox.git 
cd opensandbox
go run cmd/api/main.go
```
