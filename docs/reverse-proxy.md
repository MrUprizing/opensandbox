# Reverse Proxy

Access sandbox services via subdomain (`my-app.localhost:3000`) instead of raw Docker ports (`localhost:32768`).

## Configuration

| Env Variable  | Flag           | Default     | Description                       |
|---------------|----------------|-------------|-----------------------------------|
| `PROXY_ADDR`  | `-proxy-addr`  | `:3000`     | Proxy listen address              |
| `BASE_DOMAIN` | `-base-domain` | `localhost` | Base domain for subdomain routing |

## Creating a Sandbox with Proxy

Include `name` and `port` in the create request:

```bash
curl -X POST localhost:8080/v1/sandboxes \
  -H "Content-Type: application/json" \
  -d '{
    "image": "node:22",
    "name": "my-app",
    "port": "3000/tcp"
  }'
```

Response includes the proxy URL:

```json
{
  "id": "a1b2c3d4...",
  "ports": { "3000/tcp": "32768" },
  "url": "http://my-app.localhost:3000"
}
```

## Local Development

`*.localhost` resolves to `127.0.0.1` in modern browsers (RFC 6761). No DNS setup needed.

```bash
go run ./cmd/api
# API  → localhost:8080
# Proxy → *.localhost:3000
```

Open `http://my-app.localhost:3000` in your browser.

### If `*.localhost` doesn't resolve

Use dnsmasq for automatic wildcard resolution:

```bash
brew install dnsmasq
echo "address=/localhost/127.0.0.1" >> $(brew --prefix)/etc/dnsmasq.conf
sudo brew services start dnsmasq
sudo mkdir -p /etc/resolver
echo "nameserver 127.0.0.1" | sudo tee /etc/resolver/localhost
```

## Production

### 1. DNS

Create a wildcard A record pointing to your server:

```
*.sandbox.example.com  →  A  →  YOUR_SERVER_IP
```

### 2. Run

```bash
PROXY_ADDR=:80 \
BASE_DOMAIN=sandbox.example.com \
API_KEY=your-secret \
go run ./cmd/api
```

Sandboxes are now accessible at `http://my-app.sandbox.example.com`.

### 3. HTTPS (optional)

Place a TLS-terminating reverse proxy (Caddy, nginx) in front of the proxy server with a wildcard certificate for `*.sandbox.example.com`. The open-sandbox proxy handles plain HTTP behind it.
