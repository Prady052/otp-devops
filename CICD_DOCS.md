# CI/CD Pipeline — Line-by-Line Documentation

This document explains every file involved in the CI/CD pipeline of the **otp-devops** project.

---

## Table of Contents

1. [CI Workflow — `.github/workflows/ci.yml`](#1-ci-workflow)
2. [CD Workflow — `.github/workflows/cd.yml`](#2-cd-workflow)
3. [Deploy Script — `script/deploy.sh`](#3-deploy-script)
4. [Provision Script — `script/provision.sh`](#4-provision-script)
5. [Pipeline Flow Diagram](#5-pipeline-flow-diagram)

---

## 1. CI Workflow

**File:** `.github/workflows/ci.yml`
**Purpose:** Automatically builds and tests both backend (Go) and frontend (React) on every push or pull request to `main`.

```yaml
name: CI — Build & Test
```
> Names this workflow. This label appears in the GitHub Actions tab.

```yaml
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
```
> **Trigger conditions:**
> - `push` — runs every time code is pushed to the `main` branch
> - `pull_request` — runs when a PR targeting `main` is created or updated
> This ensures every code change is validated before merging.

---

### Backend Job

```yaml
jobs:
  backend:
    name: Backend — Go
    runs-on: ubuntu-latest
```
> - `jobs:` — defines the jobs that run in this workflow
> - `backend:` — internal identifier for this job
> - `name:` — display name in the GitHub UI
> - `runs-on: ubuntu-latest` — runs on a GitHub-hosted Ubuntu VM (free for public repos)

```yaml
    defaults:
      run:
        working-directory: backend
```
> Sets the default directory for all `run:` commands in this job to `backend/`.
> So `go build ./...` actually runs as `cd backend && go build ./...`.

```yaml
    services:
      redis:
        image: redis:7
        ports:
          - 6379:6379
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
```
> **Service container** — spins up a Redis 7 Docker container alongside the job.
> - `ports: 6379:6379` — maps container port to host so the Go app can connect
> - `options:` — Docker health check to ensure Redis is ready before tests run:
>   - `--health-cmd "redis-cli ping"` — runs `PING` command to check Redis is alive
>   - `--health-interval 10s` — check every 10 seconds
>   - `--health-timeout 5s` — timeout per check
>   - `--health-retries 5` — retry 5 times before declaring unhealthy

```yaml
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
```
> Downloads the repo code into the runner VM.
> `actions/checkout@v4` is the official GitHub Action for cloning repos.

```yaml
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: backend/go.mod
```
> Installs Go on the runner. Instead of hardcoding a version, it reads the Go version
> from `backend/go.mod` (line: `go 1.25.1`), so CI always matches your dev environment.

```yaml
      - name: Download dependencies
        run: go mod download
```
> Downloads all Go module dependencies listed in `go.mod` / `go.sum`.
> This is cached automatically by `setup-go` for faster subsequent runs.

```yaml
      - name: Vet
        run: go vet ./...
```
> `go vet` runs static analysis on all packages (`./...` means all subdirectories).
> It catches common mistakes like unreachable code, incorrect printf formats, etc.

```yaml
      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...
        env:
          REDIS_ADDR: localhost:6379
```
> Runs all Go tests with:
> - `-v` — verbose output (shows each test name and result)
> - `-race` — enables the race detector to catch concurrent bugs
> - `-coverprofile=coverage.out` — generates a code coverage report
> - `./...` — runs tests in all packages
> - `env: REDIS_ADDR` — passes Redis address so the tests can connect to the service container

```yaml
      - name: Build binary
        run: go build -o otp-server ./cmd/server
```
> Compiles the Go backend into a binary named `otp-server`.
> If this fails, there's a compilation error.

```yaml
      - name: Upload binary artifact
        uses: actions/upload-artifact@v4
        with:
          name: otp-server
          path: backend/otp-server
          retention-days: 7
```
> Uploads the compiled binary as a **GitHub artifact** (downloadable file).
> - `name: otp-server` — artifact identifier
> - `path:` — file to upload (note: path is relative to repo root, not working directory)
> - `retention-days: 7` — automatically deleted after 7 days to save storage

---

### Frontend Job

```yaml
  frontend:
    name: Frontend — React
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: frontend
```
> Same pattern as the backend job, but with `working-directory: frontend`.
> Both jobs run **in parallel** since they're independent.

```yaml
      - name: Checkout repository
        uses: actions/checkout@v4
```
> Clones the repo again (each job gets a fresh VM).

```yaml
      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: npm
          cache-dependency-path: frontend/package-lock.json
```
> Installs Node.js v20 on the runner.
> - `cache: npm` — caches `node_modules` between runs for speed
> - `cache-dependency-path:` — uses the lockfile hash as the cache key

```yaml
      - name: Install dependencies
        run: npm ci
```
> `npm ci` (clean install) installs exact versions from `package-lock.json`.
> Unlike `npm install`, it never modifies the lockfile — ideal for CI.

```yaml
      - name: Lint
        run: npm run lint
```
> Runs ESLint to check for code quality issues (like the `useEffect` error we fixed).

```yaml
      - name: Build
        run: npm run build
```
> Runs `vite build` to compile the React app into production-ready static files in `dist/`.

```yaml
      - name: Upload frontend build
        uses: actions/upload-artifact@v4
        with:
          name: frontend-dist
          path: frontend/dist
          retention-days: 7
```
> Uploads the built frontend (`dist/` folder) as a GitHub artifact.

---

## 2. CD Workflow

**File:** `.github/workflows/cd.yml`
**Purpose:** Automatically deploys the app to an LXC container on your WSL machine after CI passes.

```yaml
name: CD — Deploy
```
> Workflow name shown in GitHub Actions tab.

```yaml
on:
  workflow_dispatch:
  workflow_run:
    workflows: ["CI — Build & Test"]
    types: [completed]
    branches: [main]
```
> **Two triggers:**
> - `workflow_dispatch:` — adds a manual **"Run workflow"** button in the GitHub UI
> - `workflow_run:` — auto-triggers when the "CI — Build & Test" workflow completes on `main`
>   - `types: [completed]` — fires on both success AND failure (we filter below)

```yaml
jobs:
  deploy:
    name: Deploy to LXC (Self-Hosted)
    runs-on: self-hosted
```
> - `runs-on: self-hosted` — runs on YOUR machine (the GitHub Actions runner you installed in WSL)
> - Unlike CI which uses `ubuntu-latest` (GitHub's cloud), CD runs locally where LXC is available

```yaml
    if: ${{ github.event.workflow_run.conclusion == 'success' }}
```
> **Guard condition** — only deploys if CI succeeded.
> Without this, a failed CI would still trigger a deploy.

```yaml
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
```
> Clones the latest code onto your WSL machine.

```yaml
      - name: Make scripts executable
        run: chmod +x script/deploy.sh script/provision.sh
```
> Windows Git may strip execute permissions. This ensures the scripts can run.

```yaml
      - name: Run deployment
        run: sudo ./script/deploy.sh
```
> Runs the deploy script (explained in Section 3 below).
> `sudo` is needed because LXC container operations require root.

```yaml
      - name: Health check
        run: |
          echo "Waiting for deployment to stabilize..."
          sleep 10
          CONTAINER_IP=$(sudo lxc-info -n otp-devops -iH | head -1)
          HEALTH_URL="http://$CONTAINER_IP:8080/api/health"
          for i in $(seq 1 10); do
            if curl -sf "$HEALTH_URL" > /dev/null 2>&1; then
              echo "✓ Health check passed!"
              exit 0
            fi
            echo "Attempt $i/10 — waiting..."
            sleep 3
          done
          echo "✕ Health check failed"
          exit 1
```
> Post-deploy verification:
> - `sleep 10` — waits for services to start up
> - `lxc-info -n otp-devops -iH` — gets the container's IP address (`-i` = IP, `-H` = no header)
> - Loops 10 times, hitting `/api/health` every 3 seconds
> - `curl -sf` — silent mode, fail on HTTP errors
> - `exit 0` on success / `exit 1` on failure (marks the GitHub Action as pass/fail)

---

## 3. Deploy Script

**File:** `script/deploy.sh`
**Purpose:** Creates and configures an LXC container, copies the provision script into it, and runs it.

```bash
#!/bin/bash
```
> Shebang line — tells Linux to run this with bash.

```bash
set -euo pipefail
```
> Strict error handling:
> - `-e` — exit immediately if any command fails
> - `-u` — treat unset variables as errors
> - `-o pipefail` — a pipe fails if ANY command in the pipeline fails (not just the last)

```bash
CONTAINER_NAME="${CONTAINER_NAME:-otp-devops}"
CONTAINER_IMAGE="${CONTAINER_IMAGE:-ubuntu:24.04}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
```
> **Variables with defaults:**
> - `${VAR:-default}` — uses env var if set, otherwise uses the default value
> - `SCRIPT_DIR` — resolves the absolute path to this script's directory
> - `PROJECT_DIR` — goes one level up to get the project root

```bash
if ! command -v lxc &> /dev/null; then
    echo "[*] Installing LXC..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq lxc lxc-utils
fi
```
> `command -v lxc` — checks if the `lxc` command exists.
> If not found, installs `lxc` and `lxc-utils` packages.
> `-qq` = extra quiet (less output), `-y` = auto-yes to prompts.

```bash
if lxc-info -n "$CONTAINER_NAME" &> /dev/null; then
    sudo lxc-stop -n "$CONTAINER_NAME" --kill 2>/dev/null || true
    sudo lxc-destroy -n "$CONTAINER_NAME"
fi
```
> **Clean slate** — if a container with this name already exists:
> - `lxc-stop --kill` — force-stop the container (like pulling the power cord)
> - `|| true` — don't fail if it's already stopped
> - `lxc-destroy` — completely removes the container and its rootfs

```bash
sudo lxc-create -n "$CONTAINER_NAME" -t download -- \
    --dist ubuntu --release noble --arch amd64
```
> Creates a new LXC container:
> - `-n` — container name
> - `-t download` — use the download template (fetches from images.linuxcontainers.org)
> - `--dist ubuntu --release noble --arch amd64` — Ubuntu 24.04 (Noble Numbat), 64-bit

```bash
sudo lxc-start -n "$CONTAINER_NAME"
```
> Boots the container (like powering on a VM).

```bash
for i in $(seq 1 30); do
    CONTAINER_IP=$(sudo lxc-info -n "$CONTAINER_NAME" -iH 2>/dev/null | head -1)
    if [ -n "$CONTAINER_IP" ]; then
        echo "    Container IP: $CONTAINER_IP"
        break
    fi
    sleep 1
done
```
> Waits up to 30 seconds for the container to get a network IP from the `lxcbr0` bridge.
> `lxc-info -iH` — gets IP without headers. `head -1` — takes the first IP if multiple.

```bash
sudo lxc-attach -n "$CONTAINER_NAME" -- mkdir -p /opt/otp-devops/script
sudo cp "$SCRIPT_DIR/provision.sh" "/var/lib/lxc/$CONTAINER_NAME/rootfs/opt/otp-devops/script/provision.sh"
sudo lxc-attach -n "$CONTAINER_NAME" -- chmod +x /opt/otp-devops/script/provision.sh
```
> **Pushes the provision script into the container:**
> - `lxc-attach -- mkdir` — runs `mkdir` inside the container
> - `sudo cp ... rootfs/...` — copies the file directly into the container's filesystem
>   (every LXC container's files live under `/var/lib/lxc/<name>/rootfs/`)
> - `chmod +x` — makes it executable inside the container

```bash
sudo lxc-attach -n "$CONTAINER_NAME" -- /opt/otp-devops/script/provision.sh
```
> Runs the provision script INSIDE the container. This does all the heavy lifting (Section 4).

```bash
for i in $(seq 1 $RETRIES); do
    if curl -sf "$HEALTH_URL" > /dev/null 2>&1; then
        echo "    ✓ Health check passed!"
        ...
        exit 0
    fi
    sleep 3
done
echo "[ERROR] Health check failed after $RETRIES attempts"
exit 1
```
> Final health check — hits the Go backend's `/api/health` endpoint.
> Retries 10 times with 3-second intervals. Exits 0 (success) or 1 (failure).

---

## 4. Provision Script

**File:** `script/provision.sh`
**Purpose:** Runs INSIDE the LXC container. Installs all services, clones the repo, builds the app, and starts everything.

```bash
set -euo pipefail
```
> Same strict error handling as deploy.sh.

```bash
apt-get update -qq
apt-get upgrade -y -qq
```
> Updates the package list and upgrades all installed packages inside the container.

```bash
apt-get install -y -qq nginx
systemctl enable nginx
```
> Installs Nginx web server and enables it to start on boot.
> `systemctl enable` = start automatically when container boots.

```bash
apt-get install -y -qq redis-server
sed -i 's/^bind .*/bind 127.0.0.1 ::1/' /etc/redis/redis.conf
sed -i 's/^# maxmemory .*/maxmemory 128mb/' /etc/redis/redis.conf
sed -i 's/^# maxmemory-policy .*/maxmemory-policy allkeys-lru/' /etc/redis/redis.conf
systemctl enable redis-server
systemctl restart redis-server
```
> **Redis installation and hardening:**
> - `bind 127.0.0.1 ::1` — only accept connections from localhost (security)
> - `maxmemory 128mb` — cap memory usage at 128MB
> - `maxmemory-policy allkeys-lru` — when full, evict least-recently-used keys
> - `sed -i 's/old/new/'` — in-place search-and-replace in the config file

```bash
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
```
> **Go installation:**
> - Downloads Go tarball from the official site using `curl`
>   - `-f` = fail silently on errors, `-sS` = silent but show errors, `-L` = follow redirects
> - Extracts to `/usr/local/go` (standard Go install location)
> - Adds Go to PATH for the current session AND permanently via `/etc/profile.d/`

```bash
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
```
> **Repo setup:**
> - If `.git` exists → it's a real repo, just `git pull` to get latest changes
> - Otherwise → remove any leftover directory and fresh `git clone`
> - `--ff-only` — only fast-forward merges (fails if there are conflicts, which is safer)

```bash
cd "$APP_DIR/backend"
/usr/local/go/bin/go build -o /usr/local/bin/otp-server ./cmd/server
```
> Compiles the Go backend and places the binary at `/usr/local/bin/otp-server`.
> Uses the full Go path since `export PATH` doesn't persist into subshells.

```bash
cat > /etc/systemd/system/otp-server.service <<EOF
[Unit]
Description=OTP DevOps Server
After=network.target redis-server.service
Requires=redis-server.service
...
EOF
```
> Creates a **systemd service file** — this makes the Go backend a proper system service:
> - `After=network.target redis-server.service` — start after networking and Redis are ready
> - `Requires=redis-server.service` — if Redis stops, stop the backend too
> - `User=www-data` — runs as the web server user (not root — security)
> - `Restart=always` / `RestartSec=5` — auto-restart on crash, wait 5s between retries
> - `EnvironmentFile=` — loads env vars from `config.env`
> - `Environment=GIN_MODE=release` — disables Gin's debug logging

```bash
systemctl daemon-reload
systemctl enable otp-server
systemctl restart otp-server
```
> - `daemon-reload` — tells systemd to re-read service files
> - `enable` — start on boot
> - `restart` — start (or restart) the service now

```bash
cat > /etc/nginx/sites-available/otp-devops <<EOF
server {
    listen 80 default_server;
    ...
}
EOF
```
> **Nginx reverse proxy configuration:**
> - `listen 80` — serves HTTP on port 80
> - `root /opt/otp-devops/frontend/dist` — serves the React build
> - `location /api/` → `proxy_pass http://127.0.0.1:8080` — forwards API requests to Go backend
> - `try_files $uri $uri/ /index.html` — SPA fallback (all routes serve index.html)
> - Cache headers for static assets (JS/CSS/images) — 30-day expiry

```bash
ln -sf /etc/nginx/sites-available/otp-devops /etc/nginx/sites-enabled/default
nginx -t
systemctl restart nginx
```
> - `ln -sf` — symlinks the config to `sites-enabled/` (how Nginx activates sites)
> - `nginx -t` — tests the config for syntax errors before restarting
> - `systemctl restart nginx` — applies the new config

---

## 5. Pipeline Flow Diagram

```
 Developer pushes to main
          │
          ▼
 ┌─────────────────────────────────┐
 │     CI — GitHub Cloud ☁️         │
 │                                 │
 │  Backend Job        Frontend Job│
 │  ┌───────────┐    ┌───────────┐ │
 │  │ checkout  │    │ checkout  │ │
 │  │ setup go  │    │ setup node│ │
 │  │ go vet    │    │ npm ci    │ │
 │  │ go test   │    │ npm lint  │ │
 │  │ go build  │    │ npm build │ │
 │  │ upload    │    │ upload    │ │
 │  └───────────┘    └───────────┘ │
 │         (run in parallel)       │
 └───────────────┬─────────────────┘
                 │ CI passes ✅
                 ▼
 ┌─────────────────────────────────┐
 │     CD — Your WSL Machine 🏠    │
 │                                 │
 │  1. Checkout latest code        │
 │  2. Run deploy.sh               │
 │     ├── Destroy old container   │
 │     ├── Create new LXC          │
 │     ├── Start container         │
 │     └── Run provision.sh        │
 │         ├── Install nginx       │
 │         ├── Install redis       │
 │         ├── Install Go          │
 │         ├── Clone repo          │
 │         ├── Build backend       │
 │         ├── Create systemd svc  │
 │         └── Configure nginx     │
 │  3. Health check                │
 └─────────────────────────────────┘
                 │
                 ▼
          App is live! 🚀
   Nginx:80 → React frontend
   /api/*  → Go backend:8080
   Redis   → OTP storage
```
