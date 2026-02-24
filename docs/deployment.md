# Production Deployment Guide

Complete guide for deploying Open Sandbox on a VPS with HTTPS (wildcard TLS via Caddy + Let's Encrypt).

**Stack:** Hetzner VPS (Ubuntu 24.04) + Hetzner DNS + Caddy reverse proxy + gVisor

## Architecture

```
Internet
    │
    ▼
  Caddy (:443, TLS termination)
    │
    ├── yourdomain.com      → localhost:8080  (API server)
    │
    └── *.yourdomain.com    → localhost:3000  (Sandbox proxy)
                                    │
                                    ▼
                              Docker containers (:32xxx)
```

Caddy handles HTTPS with automatic wildcard certificates from Let's Encrypt. Open Sandbox runs behind it on plain HTTP (localhost only, not exposed to the internet).

---

## Prerequisites

- A VPS (this guide uses Hetzner Cloud, Ubuntu 24.04)
- A domain name
- Go, Docker, and gVisor installed (see [install.md](install.md))

---

## Step 1: DNS Setup

You need two A records pointing to your VPS IP. The wildcard record (`*`) makes `anything.yourdomain.com` resolve to your server.

### 1.1 Move nameservers to Hetzner DNS

If your domain is registered elsewhere (Spaceship, Namecheap, etc.), change the nameservers to Hetzner:

```
hydrogen.ns.hetzner.com
oxygen.ns.hetzner.com
helium.ns.hetzner.de
```

This delegates DNS resolution to Hetzner while your domain stays registered at the original registrar.

### 1.2 Create DNS zone in Hetzner

Go to the Hetzner Cloud Console DNS section and create a zone for your domain. Then add the A records:

| Type | Name | Value            | TTL |
|------|------|------------------|-----|
| A    | `@`  | `YOUR_SERVER_IP` | 300 |
| A    | `*`  | `YOUR_SERVER_IP` | 300 |

### 1.3 Verify DNS propagation

Wait a few minutes after adding the records, then verify:

```bash
dig yourdomain.com +short
# Should return: YOUR_SERVER_IP

dig test.yourdomain.com +short
# Should also return: YOUR_SERVER_IP
```

Both must return your server IP before proceeding.

---

## Step 2: Hetzner Cloud API Token

Caddy needs an API token to create temporary DNS TXT records for the Let's Encrypt DNS-01 challenge (required for wildcard certs).

1. Go to Hetzner Cloud Console → your project → **Security** → **API Tokens**
2. Create a new token with **Read & Write** permissions
3. Copy and save the token

> **Important:** This is a Hetzner **Cloud** API token (from `console.hetzner.cloud`), not the old DNS Console token. Hetzner migrated DNS management to the Cloud Console — the old `dns.hetzner.com` tokens no longer work.

Verify the token works:

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://api.hetzner.cloud/v1/zones
```

Should return a JSON with your DNS zone.

---

## Step 3: Install Caddy with Hetzner DNS Plugin

The default Caddy binary doesn't include DNS provider plugins. You need to compile a custom build with the Hetzner plugin using `xcaddy`.

> **Important:** Use **v2** of the plugin (`github.com/caddy-dns/hetzner/v2`). Version 1 uses the deprecated `dns.hetzner.com` API. Version 2 uses the current Hetzner Cloud API.

```bash
# Install xcaddy
go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest

# Build Caddy with Hetzner DNS plugin v2 (~1-2 minutes)
~/go/bin/xcaddy build --with github.com/caddy-dns/hetzner/v2

# Verify
./caddy version

# Move to system PATH
sudo mv caddy /usr/local/bin/caddy
```

---

## Step 4: Configure Caddy

### 4.1 Create the Caddyfile

```bash
sudo mkdir -p /etc/caddy

sudo tee /etc/caddy/Caddyfile > /dev/null << 'EOF'
yourdomain.com {
    reverse_proxy localhost:8080
}

*.yourdomain.com {
    reverse_proxy localhost:3000

    tls {
        dns hetzner {env.HETZNER_DNS_API_TOKEN}
        propagation_delay 30s
    }
}
EOF
```

Replace `yourdomain.com` with your actual domain.

The `propagation_delay 30s` tells Caddy to wait 30 seconds before checking DNS propagation, which avoids failures caused by slow DNS updates.

### 4.2 Create systemd service

Create the system user for Caddy:

```bash
sudo groupadd --system caddy 2>/dev/null
sudo useradd --system --gid caddy --create-home \
  --home-dir /var/lib/caddy --shell /usr/sbin/nologin caddy 2>/dev/null
```

Create the service file:

```bash
sudo tee /etc/systemd/system/caddy.service > /dev/null << 'EOF'
[Unit]
Description=Caddy web server
After=network.target network-online.target
Requires=network-online.target

[Service]
Type=notify
User=caddy
Group=caddy
Environment=HETZNER_DNS_API_TOKEN=YOUR_TOKEN_HERE
ExecStart=/usr/local/bin/caddy run --environ --config /etc/caddy/Caddyfile
ExecReload=/usr/local/bin/caddy reload --config /etc/caddy/Caddyfile
TimeoutStopSec=5s
LimitNOFILE=1048576
PrivateTmp=true
ProtectSystem=full
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
EOF
```

Replace `YOUR_TOKEN_HERE` with your Hetzner Cloud API token:

```bash
sudo sed -i 's/YOUR_TOKEN_HERE/paste_your_real_token/' /etc/systemd/system/caddy.service
```

### 4.3 Start Caddy

```bash
sudo systemctl daemon-reload
sudo systemctl enable caddy
sudo systemctl start caddy
```

Verify it's running:

```bash
sudo systemctl status caddy
```

Should show `active (running)`. Check logs for certificate issuance:

```bash
sudo journalctl -u caddy --no-pager -n 30
```

Look for `"certificate obtained successfully"`. The first wildcard cert takes ~30-60 seconds due to the DNS-01 challenge.

---

## Step 5: Firewall

Only expose ports 80 (HTTP redirect), 443 (HTTPS), and 22 (SSH). The API (`:8080`) and proxy (`:3000`) are only accessible via Caddy on localhost.

```bash
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

Verify:

```bash
sudo ufw status
```

Expected output:

```
To                         Action      From
--                         ------      ----
22/tcp                     ALLOW       Anywhere
80/tcp                     ALLOW       Anywhere
443/tcp                    ALLOW       Anywhere
```

---

## Step 6: Run Open Sandbox

```bash
BASE_DOMAIN=yourdomain.com \
ADDR=:8080 \
PROXY_ADDR=:3000 \
API_KEY=your-secret-key \
go run ./cmd/api
```

---

## Step 7: Verify

### API health check

```bash
curl https://yourdomain.com/v1/health
# {"status":"healthy"}
```

### Create a sandbox

```bash
curl -X POST https://yourdomain.com/v1/sandboxes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-secret-key" \
  -d '{"image": "node:20-alpine", "port": "3000"}'
```

### Access sandbox via HTTPS

Using the `name` from the create response (e.g. `eager-turing`):

```bash
curl https://eager-turing.yourdomain.com
```

---

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `curl: (7) Failed to connect` on :443 | Firewall blocking | `sudo ufw allow 443/tcp` |
| Caddy: `Unauthorized (401)` on DNS challenge | Wrong API token or wrong token type | Must be a Hetzner **Cloud** token (from `console.hetzner.cloud`), not old DNS Console token |
| Caddy: `tls: no matching certificate` | Wildcard cert not yet issued | Wait ~60s, check `journalctl -u caddy` |
| `no subdomain in request` | `BASE_DOMAIN` doesn't match domain | Verify the env var matches your domain exactly |
| API returns 502 | Open Sandbox not running on `:8080` | Start the application |
| Sandbox returns 502 | Container not running or wrong port | Check sandbox status and port mapping |
| `dial tcp [::1]:8080: connection refused` in Caddy logs | Open Sandbox process not running | Start the application |
| DNS not resolving | Nameservers not propagated yet | Wait and retry `dig yourdomain.com +short` |

### Useful commands

```bash
# Caddy logs (live)
sudo journalctl -u caddy -f

# Restart Caddy after config changes
sudo systemctl restart caddy

# Check if Caddy owns the certificate
curl -vI https://yourdomain.com 2>&1 | grep "issuer"

# Check wildcard cert
curl -vI https://test.yourdomain.com 2>&1 | grep "issuer"
```

---

## Notes

- **Caddy auto-renews certificates.** No cron jobs or manual renewal needed.
- **HTTP is automatically redirected to HTTPS** by Caddy.
- **The proxy server (`:3000`) has no authentication.** Anyone with a valid sandbox name can access it. Consider adding proxy auth for production (see IMPROVEMENTS_2.md).
- **Timers are in-memory.** If the process restarts, auto-stop timers are lost. See IMPROVEMENTS_2.md for the persistence fix.
- **The `url` field in API responses** currently generates `http://` URLs with the internal proxy port. When running behind Caddy, the actual URL is `https://name.yourdomain.com`. A config option for this is planned.
