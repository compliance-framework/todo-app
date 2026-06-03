#!/usr/bin/env bash
set -euo pipefail

# Bootstraps the CCF (Compliance Framework) UI + API containers on the app host,
# alongside the todo-app systemd service. Reads configuration from
# /etc/ccf/ccf.env (written by user_data), creates the CCF database on the shared
# RDS instance, and runs the stack under Docker Compose with a systemd unit so it
# survives reboots. Idempotent: safe to re-run.

CCF_ENV_FILE="${CCF_ENV_FILE:-/etc/ccf/ccf.env}"
if [ -f "$CCF_ENV_FILE" ]; then
  # shellcheck disable=SC1090
  set -a
  . "$CCF_ENV_FILE"
  set +a
fi

CCF_HOME="${CCF_HOME:-/opt/ccf}"
AWS_REGION="${AWS_REGION:-eu-west-2}"
CCF_DOMAIN_NAME="${CCF_DOMAIN_NAME:-}"
CCF_API_IMAGE="${CCF_API_IMAGE:-ghcr.io/compliance-framework/api:0.16.2}"
CCF_UI_IMAGE="${CCF_UI_IMAGE:-ghcr.io/compliance-framework/ui:2.9.2}"
CCF_API_HOST_PORT="${CCF_API_HOST_PORT:-8081}"
CCF_UI_HOST_PORT="${CCF_UI_HOST_PORT:-3000}"
CCF_DB_NAME="${CCF_DB_NAME:-ccf}"
DB_HOST="${DB_HOST:-}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-}"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_PASSWORD_SECRET_ARN="${DB_PASSWORD_SECRET_ARN:-}"
JWT_SECRET="${JWT_SECRET:-}"
JWT_SECRET_ARN="${JWT_SECRET_ARN:-}"
SSO_GOOGLE_CLIENT_ID="${SSO_GOOGLE_CLIENT_ID:-}"
SSO_GOOGLE_CLIENT_SECRET="${SSO_GOOGLE_CLIENT_SECRET:-}"
SSO_GOOGLE_CLIENT_SECRET_ARN="${SSO_GOOGLE_CLIENT_SECRET_ARN:-}"
SSO_GOOGLE_HOSTED_DOMAIN="${SSO_GOOGLE_HOSTED_DOMAIN:-}"
SSO_ADMIN_EMAIL="${SSO_ADMIN_EMAIL:-}"
SSO_DOMAIN_ADMINS="${SSO_DOMAIN_ADMINS:-false}"
COMPOSE_VERSION="${COMPOSE_VERSION:-v2.32.4}"
# Maintenance database used only to issue CREATE DATABASE; RDS always provides it.
BOOTSTRAP_DB="${BOOTSTRAP_DB:-postgres}"

log() {
  printf '[ccf-bootstrap] %s\n' "$*"
}

require() {
  if [ -z "${2:-}" ]; then
    log "missing required value: $1"
    exit 1
  fi
}

install_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    log "installing docker"
    if command -v dnf >/dev/null 2>&1; then
      dnf install -y docker
    elif command -v yum >/dev/null 2>&1; then
      yum install -y docker
    else
      log "no supported package manager for docker (expected dnf or yum)"
      exit 1
    fi
  fi
  systemctl enable docker
  systemctl start docker
}

install_compose_plugin() {
  local plugin_dir="/usr/libexec/docker/cli-plugins"
  local plugin_path="$plugin_dir/docker-compose"
  if docker compose version >/dev/null 2>&1; then
    log "docker compose plugin already present"
    return
  fi

  local arch
  case "$(uname -m)" in
    x86_64) arch="x86_64" ;;
    aarch64 | arm64) arch="aarch64" ;;
    *) log "unsupported architecture for docker compose: $(uname -m)"; exit 1 ;;
  esac

  log "installing docker compose plugin ${COMPOSE_VERSION}"
  install -d -m 0755 "$plugin_dir"
  curl --fail --location --silent --show-error --retry 5 --retry-delay 2 --retry-all-errors \
    --connect-timeout 10 --max-time 180 \
    "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-${arch}" \
    --output "$plugin_path"
  chmod 0755 "$plugin_path"
  docker compose version
}

fetch_secret() {
  aws secretsmanager get-secret-value \
    --secret-id "$1" \
    --query SecretString \
    --output text \
    --region "$AWS_REGION"
}

resolve_secrets() {
  if [ -n "$DB_PASSWORD_SECRET_ARN" ]; then
    DB_PASSWORD="$(fetch_secret "$DB_PASSWORD_SECRET_ARN")"
  fi
  require DB_PASSWORD "$DB_PASSWORD"

  if [ -n "$JWT_SECRET_ARN" ]; then
    JWT_SECRET="$(fetch_secret "$JWT_SECRET_ARN")"
  fi
  require JWT_SECRET "$JWT_SECRET"

  if [ -n "$SSO_GOOGLE_CLIENT_SECRET_ARN" ]; then
    SSO_GOOGLE_CLIENT_SECRET="$(fetch_secret "$SSO_GOOGLE_CLIENT_SECRET_ARN")"
  fi
}

sso_enabled() {
  [ -n "$SSO_GOOGLE_CLIENT_ID" ] && [ -n "$SSO_GOOGLE_CLIENT_SECRET" ]
}

# Renders an sso.yaml mirroring the local-dev Google SSO config, with URLs
# pointed at the CCF hostname. Only written when Google credentials are present.
write_sso_config() {
  if ! sso_enabled; then
    log "CCF SSO not configured (no Google client id/secret); skipping sso.yaml"
    return
  fi

  log "writing CCF SSO config (Google)"
  cat >"$CCF_HOME/sso.yaml" <<EOF
# Rendered by ccf-bootstrap.sh — Google-only SSO, mirroring local-dev.
enabled: true
base_url: "https://${CCF_DOMAIN_NAME}"
callback_url: "https://${CCF_DOMAIN_NAME}/api/auth/sso/callback"

providers:
  google:
    name: "google"
    display_name: "Google"
    provider: "google"
    protocol: "oidc"
    icon_url: "https://www.gstatic.com/firebasejs/ui/2.0.0/images/auth/google.svg"
    required_login_groups:
      - "ccf-authorized-users"
    required_admin_groups:
      - "ccf-admins"
    client_id: "${SSO_GOOGLE_CLIENT_ID}"
    client_secret: "${SSO_GOOGLE_CLIENT_SECRET}"
    issuer_url: "https://accounts.google.com"
    scopes:
      - "openid"
      - "email"
      - "profile"
    enabled: true
    group_mapping:
      "hd:${SSO_GOOGLE_HOSTED_DOMAIN}":
        - "ccf-authorized-users"
EOF

  if [ "$SSO_DOMAIN_ADMINS" = "true" ]; then
    cat >>"$CCF_HOME/sso.yaml" <<EOF
        - "ccf-admins"
EOF
  fi

  if [ -n "$SSO_ADMIN_EMAIL" ]; then
    cat >>"$CCF_HOME/sso.yaml" <<EOF
      "email:${SSO_ADMIN_EMAIL}":
        - "ccf-admins"
EOF
  fi

  chmod 0600 "$CCF_HOME/sso.yaml"
}

install_psql_client() {
  if command -v psql >/dev/null 2>&1; then
    return
  fi
  log "installing postgresql client"
  # Lightweight client only (no server) — used solely to issue CREATE DATABASE
  # against the shared RDS instance. AL2023 ships postgresql15/16; either works
  # as a client against the PG17 server for this simple command.
  if command -v dnf >/dev/null 2>&1; then
    dnf install -y postgresql16 || dnf install -y postgresql15 || dnf install -y postgresql
  elif command -v yum >/dev/null 2>&1; then
    yum install -y postgresql16 || yum install -y postgresql15 || yum install -y postgresql
  else
    log "no supported package manager for the postgresql client"
    exit 1
  fi
}

create_ccf_database() {
  log "ensuring database '${CCF_DB_NAME}' exists on ${DB_HOST}"
  local exists
  exists="$(PGPASSWORD="$DB_PASSWORD" PGSSLMODE=require \
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$BOOTSTRAP_DB" -tAc \
    "SELECT 1 FROM pg_database WHERE datname = '${CCF_DB_NAME}'" 2>/dev/null || true)"

  if [ "$exists" = "1" ]; then
    log "database '${CCF_DB_NAME}' already exists"
    return
  fi

  PGPASSWORD="$DB_PASSWORD" PGSSLMODE=require \
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$BOOTSTRAP_DB" -c \
    "CREATE DATABASE \"${CCF_DB_NAME}\""
  log "database '${CCF_DB_NAME}' created"
}

write_config() {
  install -d -m 0755 "$CCF_HOME"

  # API_URL is the origin only (no /api). The UI prepends /api to its request
  # paths, and the ALB routes /api/* to the API target group.
  cat >"$CCF_HOME/config.json" <<EOF
{
  "API_URL": "https://${CCF_DOMAIN_NAME}"
}
EOF
  chmod 0644 "$CCF_HOME/config.json"

  cat >"$CCF_HOME/api.env" <<EOF
CCF_ENVIRONMENT=production
CCF_DB_DRIVER=postgres
CCF_DB_CONNECTION=host=${DB_HOST} user=${DB_USER} password=${DB_PASSWORD} dbname=${CCF_DB_NAME} port=${DB_PORT} sslmode=require
CCF_API_ALLOWED_ORIGINS=https://${CCF_DOMAIN_NAME}
CCF_WEB_BASE_URL=https://${CCF_DOMAIN_NAME}
CCF_JWT_SECRET=${JWT_SECRET}
EOF

  # When SSO is configured, point the API at the rendered sso.yaml and mount it.
  local api_volumes=""
  if sso_enabled; then
    printf 'CCF_SSO_CONFIG=/sso.yaml\n' >>"$CCF_HOME/api.env"
    api_volumes=$'    volumes:\n      - '"${CCF_HOME}/sso.yaml:/sso.yaml:ro"
  fi
  chmod 0600 "$CCF_HOME/api.env"

  cat >"$CCF_HOME/docker-compose.yml" <<EOF
services:
  ccf-api:
    image: ${CCF_API_IMAGE}
    restart: always
    env_file:
      - ${CCF_HOME}/api.env
${api_volumes:+$api_volumes
}    ports:
      - "${CCF_API_HOST_PORT}:8080"

  ccf-ui:
    image: ${CCF_UI_IMAGE}
    restart: always
    depends_on:
      - ccf-api
    volumes:
      - ${CCF_HOME}/config.json:/app/config.json:ro
    ports:
      - "${CCF_UI_HOST_PORT}:80"
EOF
  chmod 0644 "$CCF_HOME/docker-compose.yml"
}

write_systemd_unit() {
  cat >/etc/systemd/system/ccf.service <<EOF
[Unit]
Description=CCF (UI + API) docker compose stack
Requires=docker.service
After=docker.service network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=${CCF_HOME}
ExecStart=/usr/bin/docker compose -f ${CCF_HOME}/docker-compose.yml up -d
ExecStop=/usr/bin/docker compose -f ${CCF_HOME}/docker-compose.yml down

[Install]
WantedBy=multi-user.target
EOF
  chmod 0644 /etc/systemd/system/ccf.service
  systemctl daemon-reload
}

main() {
  require CCF_DOMAIN_NAME "$CCF_DOMAIN_NAME"
  require DB_HOST "$DB_HOST"
  require DB_USER "$DB_USER"

  install_docker
  install_compose_plugin
  resolve_secrets

  install_psql_client
  create_ccf_database

  install -d -m 0755 "$CCF_HOME"
  write_sso_config
  write_config
  write_systemd_unit

  docker compose -f "$CCF_HOME/docker-compose.yml" pull
  systemctl enable ccf.service
  systemctl restart ccf.service
  log "CCF stack started"
}

main "$@"
