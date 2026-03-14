#!/bin/bash
# deploy.sh — Automated LXC container deployment
# Run from the WSL2 host

set -euo pipefail

CONTAINER_NAME="${CONTAINER_NAME:-otp-devops}"
CONTAINER_IMAGE="${CONTAINER_IMAGE:-ubuntu:24.04}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "========================================="
echo "  OTP-DevOps — Deployment Script"
echo "========================================="
echo "Container: $CONTAINER_NAME"
echo "Image:     $CONTAINER_IMAGE"
echo ""

# ---- Install LXC if needed ----
if ! command -v lxc &> /dev/null; then
    echo "[*] Installing LXC..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq lxc lxc-utils
fi

# ---- Create or recreate container ----
if lxc-info -n "$CONTAINER_NAME" &> /dev/null; then
    echo "[*] Stopping existing container..."
    sudo lxc-stop -n "$CONTAINER_NAME" --kill 2>/dev/null || true
    echo "[*] Destroying existing container..."
    sudo lxc-destroy -n "$CONTAINER_NAME"
fi

echo "[*] Creating container..."
sudo lxc-create -n "$CONTAINER_NAME" -t download -- \
    --dist ubuntu --release noble --arch amd64

echo "[*] Starting container..."
sudo lxc-start -n "$CONTAINER_NAME"

# Wait for container networking
echo "[*] Waiting for container network..."
for i in $(seq 1 30); do
    CONTAINER_IP=$(sudo lxc-info -n "$CONTAINER_NAME" -iH 2>/dev/null | head -1)
    if [ -n "$CONTAINER_IP" ]; then
        echo "    Container IP: $CONTAINER_IP"
        break
    fi
    sleep 1
done

if [ -z "$CONTAINER_IP" ]; then
    echo "[ERROR] Container failed to get an IP address"
    exit 1
fi

# ---- Push provisioning script ----
echo "[*] Pushing provision script..."
sudo lxc-attach -n "$CONTAINER_NAME" -- mkdir -p /opt/otp-devops/script
sudo cp "$SCRIPT_DIR/provision.sh" "/var/lib/lxc/$CONTAINER_NAME/rootfs/opt/otp-devops/script/provision.sh"
sudo lxc-attach -n "$CONTAINER_NAME" -- chmod +x /opt/otp-devops/script/provision.sh

# ---- Run provisioning ----
echo "[*] Running provisioning inside container..."
sudo lxc-attach -n "$CONTAINER_NAME" -- /opt/otp-devops/script/provision.sh

# ---- Health check ----
echo ""
echo "[*] Running health check..."
HEALTH_URL="http://$CONTAINER_IP:8080/api/health"
RETRIES=10
for i in $(seq 1 $RETRIES); do
    if curl -sf "$HEALTH_URL" > /dev/null 2>&1; then
        echo "    ✓ Health check passed!"
        echo ""
        echo "========================================="
        echo "  Deployment successful!"
        echo "  Container IP: $CONTAINER_IP"
        echo "  Health:    $HEALTH_URL"
        echo "  App:       http://$CONTAINER_IP"
        echo "========================================="
        exit 0
    fi
    echo "    Attempt $i/$RETRIES — waiting..."
    sleep 3
done

echo "[ERROR] Health check failed after $RETRIES attempts"
exit 1
