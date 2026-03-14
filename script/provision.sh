#!/bin/bash
# provision.sh — Provisions the LXC container with all dependencies
# Run inside the LXC container (not on the host)

set -euo pipefail

echo "========================================="
echo "  OTP-DevOps — Container Provisioning"
echo "========================================="

# ---- System updates ----
echo "[1/7] Updating system packages..."
apt-get update -qq
apt-get upgrade -y -qq

# ---- Install Nginx ----
echo "[2/7] Installing Nginx..."
apt-get install -y -qq nginx
systemctl enable nginx

# ---- Install Redis ----
echo "[3/7] Installing Redis..."
apt-get install -y -qq redis-server
# Bind to localhost only for security
sed -i 's/^bind .*/bind 127.0.0.1 ::1/' /etc/redis/redis.conf
sed -i 's/^# maxmemory .*/maxmemory 128mb/' /etc/redis/redis.conf
sed -i 's/^# maxmemory-policy .*/maxmemory-policy allkeys-lru/' /etc/redis/redis.conf
systemctl enable redis-server
systemctl restart redis-server

# ---- Install Go ----
echo "[4/7] Installing Go..."
apt-get install -y -qq wget curl
GO_VERSION="1.23.4"
if ! command -v go &> /dev/null; then
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
fi
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh

# ---- Install Git & clone repo ----
echo "[5/7] Setting up application..."
apt-get install -y -qq git

APP_DIR="/opt/otp-devops"
REPO_URL="${REPO_URL:-https://github.com/Prady052/otp-devops.git}"

if [ -d "$APP_DIR/.git" ]; then
    cd "$APP_DIR"
    git pull --ff-only
else
    rm -rf "$APP_DIR"
    git clone "$REPO_URL" "$APP_DIR"
    cd "$APP_DIR"
fi

# ---- Build backend ----
echo "[6/7] Building backend..."
cd "$APP_DIR/backend"
/usr/local/go/bin/go build -o /usr/local/bin/otp-server ./cmd/server

# ---- Create systemd service ----
echo "[7/7] Setting up systemd service..."
cat > /etc/systemd/system/otp-server.service <<EOF
[Unit]
Description=OTP DevOps Server
After=network.target redis-server.service
Requires=redis-server.service

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/opt/otp-devops/backend
ExecStart=/usr/local/bin/otp-server
Restart=always
RestartSec=5
EnvironmentFile=/opt/otp-devops/config/config.env
Environment=GIN_MODE=release

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable otp-server
systemctl restart otp-server

# ---- Configure Nginx ----
cat > /etc/nginx/sites-available/otp-devops <<EOF
server {
    listen 80 default_server;
    server_name _;

    # Frontend static files (built React app)
    root /opt/otp-devops/frontend/dist;
    index index.html;

    # API proxy to Go backend
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 30s;
    }

    # Health check endpoint
    location /api/health {
        proxy_pass http://127.0.0.1:8080;
    }

    # SPA fallback — serve index.html for all non-file routes
    location / {
        try_files \$uri \$uri/ /index.html;
    }

    # Cache static assets
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff2?)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
    }
}
EOF

ln -sf /etc/nginx/sites-available/otp-devops /etc/nginx/sites-enabled/default
nginx -t
systemctl restart nginx

echo ""
echo "========================================="
echo "  Provisioning complete!"
echo "  Backend:  http://localhost:8080/api/health"
echo "  Frontend: http://localhost"
echo "========================================="
