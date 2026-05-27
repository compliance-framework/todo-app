#!/usr/bin/env bash
set -euo pipefail

BOOTSTRAP_ENV_FILE="${BOOTSTRAP_ENV_FILE:-/etc/todo-app/bootstrap.env}"
if [ -f "$BOOTSTRAP_ENV_FILE" ]; then
  # shellcheck disable=SC1090
  set -a
  . "$BOOTSTRAP_ENV_FILE"
  set +a
fi

APP_HOME="${APP_HOME:-/opt/todo-app}"
APP_USER="${APP_USER:-todoapp}"
APP_GROUP="${APP_GROUP:-todoapp}"
ENV_FILE="${ENV_FILE:-/etc/todo-app/todo-app.env}"
SERVICE_FILE="${SERVICE_FILE:-/etc/systemd/system/todo-app.service}"
AWS_REGION="${AWS_REGION:-eu-west-2}"
APP_PORT="${APP_PORT:-8080}"
FALLBACK_RELEASE_TAG="${FALLBACK_RELEASE_TAG:-v0.1.0}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-compliance-framework/todo-app}"
RELEASE_ARTIFACT_NAME="${RELEASE_ARTIFACT_NAME:-todo-app-linux-amd64}"
RELEASE_SIGNATURE_BUNDLE_NAME="${RELEASE_SIGNATURE_BUNDLE_NAME:-todo-app-linux-amd64.bundle}"
SKIP_COSIGN_VERIFY="${SKIP_COSIGN_VERIFY:-false}"
COSIGN_VERSION="${COSIGN_VERSION:-v2.4.3}"
COSIGN_LINUX_AMD64_SHA256="${COSIGN_LINUX_AMD64_SHA256:-}"
COSIGN_LINUX_ARM64_SHA256="${COSIGN_LINUX_ARM64_SHA256:-}"
DB_HOST="${DB_HOST:-}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-tododb}"
DB_USER="${DB_USER:-todoapp}"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_PASSWORD_SECRET_ARN="${DB_PASSWORD_SECRET_ARN:-}"
DB_RDS_CA_BUNDLE_PATH="${DB_RDS_CA_BUNDLE_PATH:-/opt/rds-ca/global-bundle.pem}"
DB_RDS_CA_BUNDLE_URL="${DB_RDS_CA_BUNDLE_URL:-https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem}"
JWT_SECRET="${JWT_SECRET:-}"
JWT_SECRET_ARN="${JWT_SECRET_ARN:-}"
OIDC_ISSUER_URL="${OIDC_ISSUER_URL:-}"
OIDC_CLIENT_ID="${OIDC_CLIENT_ID:-}"
OIDC_CLIENT_SECRET="${OIDC_CLIENT_SECRET:-}"
OIDC_CLIENT_SECRET_ARN="${OIDC_CLIENT_SECRET_ARN:-}"
OIDC_REDIRECT_URL="${OIDC_REDIRECT_URL:-}"
OIDC_FRONTEND_URL="${OIDC_FRONTEND_URL:-}"
OIDC_COOKIE_SECURE="${OIDC_COOKIE_SECURE:-true}"
COSIGN_CERTIFICATE_IDENTITY_REGEXP="${COSIGN_CERTIFICATE_IDENTITY_REGEXP:-https://github.com/compliance-framework/todo-app/.github/workflows/.*}"
COSIGN_CERTIFICATE_OIDC_ISSUER="${COSIGN_CERTIFICATE_OIDC_ISSUER:-https://token.actions.githubusercontent.com}"

log() {
  printf '[todo-app-bootstrap] %s\n' "$*"
}

install_packages() {
  if command -v dnf >/dev/null 2>&1; then
    dnf install -y --allowerasing awscli curl shadow-utils
  elif command -v yum >/dev/null 2>&1; then
    yum install -y awscli curl shadow-utils
  elif command -v apt-get >/dev/null 2>&1; then
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y awscli curl
  else
    log "no supported package manager found; expected dnf, yum, or apt-get"
    exit 1
  fi
}

install_rds_ca_bundle() {
  log "downloading RDS CA bundle"
  install -d -m 0755 "$(dirname "$DB_RDS_CA_BUNDLE_PATH")"
  curl --fail --location --silent --show-error --retry 5 --retry-delay 2 --connect-timeout 10 --max-time 120 "$DB_RDS_CA_BUNDLE_URL" --output "$DB_RDS_CA_BUNDLE_PATH"
  chmod 0644 "$DB_RDS_CA_BUNDLE_PATH"
}

install_cosign() {
  local arch
  local checksum
  arch="$(uname -m)"
  case "$arch" in
    x86_64)
      arch="amd64"
      checksum="$COSIGN_LINUX_AMD64_SHA256"
      ;;
    aarch64 | arm64)
      arch="arm64"
      checksum="$COSIGN_LINUX_ARM64_SHA256"
      ;;
    *) log "unsupported architecture for cosign: $arch"; exit 1 ;;
  esac

  if [ -z "$checksum" ]; then
    log "missing pinned checksum for cosign ${COSIGN_VERSION} linux ${arch}"
    exit 1
  fi

  local url
  local work_dir
  local cosign_file
  url="https://github.com/sigstore/cosign/releases/download/${COSIGN_VERSION}/cosign-linux-${arch}"
  work_dir="$(mktemp -d)"
  cosign_file="$work_dir/cosign"

  (
    trap 'rm -rf "$work_dir"' EXIT

    log "installing cosign ${COSIGN_VERSION}"
    curl --fail --location --silent --show-error --retry 5 --retry-delay 2 --retry-all-errors --connect-timeout 10 --max-time 120 "$url" --output "$cosign_file"
    printf '%s  %s\n' "$checksum" "$cosign_file" | sha256sum --check --status
    install -o root -g root -m 0755 "$cosign_file" /usr/local/bin/cosign
  )
}

ensure_user() {
  if ! getent group "$APP_GROUP" >/dev/null; then
    groupadd --system "$APP_GROUP"
  fi

  if ! id "$APP_USER" >/dev/null 2>&1; then
    useradd --system --gid "$APP_GROUP" --home-dir /var/lib/todo-app --shell /sbin/nologin "$APP_USER"
  fi

  install -d -o "$APP_USER" -g "$APP_GROUP" -m 0750 /var/lib/todo-app
  install -d -o root -g root -m 0755 "$APP_HOME/bin" "$APP_HOME/releases"
  install -d -o root -g "$APP_GROUP" -m 0750 /etc/todo-app
}

download_and_verify() {
  local tag="$1"
  local work_dir="$2"
  local artifact="$work_dir/$RELEASE_ARTIFACT_NAME"
  local bundle="$work_dir/$RELEASE_SIGNATURE_BUNDLE_NAME"
  local base_url="https://github.com/${GITHUB_REPOSITORY}/releases/download/${tag}"

  log "downloading release ${tag}"
  curl --fail --location --silent --show-error --retry 5 --retry-delay 2 --connect-timeout 10 --max-time 120 "${base_url}/${RELEASE_ARTIFACT_NAME}" --output "$artifact"

  if [ "$SKIP_COSIGN_VERIFY" = "true" ]; then
    log "skipping cosign verification (SKIP_COSIGN_VERIFY=true)"
  else
    curl --fail --location --silent --show-error --retry 5 --retry-delay 2 --connect-timeout 10 --max-time 120 "${base_url}/${RELEASE_SIGNATURE_BUNDLE_NAME}" --output "$bundle"
    log "verifying sigstore signature for ${RELEASE_ARTIFACT_NAME}"
    cosign verify-blob \
      --bundle "$bundle" \
      --certificate-identity-regexp "$COSIGN_CERTIFICATE_IDENTITY_REGEXP" \
      --certificate-oidc-issuer "$COSIGN_CERTIFICATE_OIDC_ISSUER" \
      "$artifact"
  fi

  chmod 0755 "$artifact"
}

invalid_release_tag() {
  log "invalid release tag: $(printf '%q' "$1")"
  exit 1
}

validate_release_tag() {
  local tag="$1"

  if [[ -z "$tag" ||
    "$tag" == "." ||
    "$tag" == ".." ||
    "$tag" == *".."* ||
    "$tag" == *"/"* ||
    "$tag" =~ [[:space:]] ||
    ! "$tag" =~ ^[A-Za-z0-9._-]+$ ]]; then
    invalid_release_tag "$tag"
  fi
}

fetch_secret() {
  aws secretsmanager get-secret-value \
    --secret-id "$1" \
    --query SecretString \
    --output text \
    --region "$AWS_REGION"
}

write_environment_file() {
  if [ -n "$DB_PASSWORD_SECRET_ARN" ]; then
    DB_PASSWORD="$(fetch_secret "$DB_PASSWORD_SECRET_ARN")"
  fi

  if [ -z "$DB_PASSWORD" ]; then
    log "missing DB password; set DB_PASSWORD_SECRET_ARN or DB_PASSWORD"
    exit 1
  fi

  if [ -n "$JWT_SECRET_ARN" ]; then
    JWT_SECRET="$(fetch_secret "$JWT_SECRET_ARN")"
  fi

  if [ -z "$JWT_SECRET" ]; then
    log "missing JWT secret; set JWT_SECRET_ARN or JWT_SECRET"
    exit 1
  fi

  if [ -n "$OIDC_CLIENT_SECRET_ARN" ]; then
    OIDC_CLIENT_SECRET="$(fetch_secret "$OIDC_CLIENT_SECRET_ARN")"
  fi

  if [ -z "$DB_HOST" ]; then
    log "missing DB_HOST"
    exit 1
  fi

  if [ -z "$DB_PORT" ]; then
    log "missing DB_PORT"
    exit 1
  fi

  if ! [[ "$DB_PORT" =~ ^[0-9]+$ ]] || [ "$DB_PORT" -lt 1 ] || [ "$DB_PORT" -gt 65535 ]; then
    log "invalid DB_PORT: $DB_PORT"
    exit 1
  fi

  if [ -z "$DB_NAME" ]; then
    log "missing DB_NAME"
    exit 1
  fi

  if [ -z "$DB_USER" ]; then
    log "missing DB_USER"
    exit 1
  fi

  cat >"$ENV_FILE" <<EOF
PORT=${APP_PORT}
JWT_SECRET=${JWT_SECRET}
DB_DRIVER=postgres
DB_HOST=${DB_HOST}
DB_PORT=${DB_PORT}
DB_NAME=${DB_NAME}
DB_USER=${DB_USER}
DB_PASSWORD=${DB_PASSWORD}
DB_IAM_AUTH=false
DB_REGION=${AWS_REGION}
DB_SSLROOTCERT=${DB_RDS_CA_BUNDLE_PATH}
EOF

  if [ -n "$OIDC_CLIENT_ID" ] && [ -n "$OIDC_CLIENT_SECRET" ]; then
    cat >>"$ENV_FILE" <<EOF
OIDC_ISSUER_URL=${OIDC_ISSUER_URL}
OIDC_CLIENT_ID=${OIDC_CLIENT_ID}
OIDC_CLIENT_SECRET=${OIDC_CLIENT_SECRET}
OIDC_REDIRECT_URL=${OIDC_REDIRECT_URL}
OIDC_FRONTEND_URL=${OIDC_FRONTEND_URL}
OIDC_COOKIE_SECURE=${OIDC_COOKIE_SECURE}
EOF
  else
    log "OIDC not fully configured; skipping OIDC env vars"
  fi

  chown root:"$APP_GROUP" "$ENV_FILE"
  chmod 0640 "$ENV_FILE"
}

write_systemd_unit() {
  cat >"$SERVICE_FILE" <<EOF
[Unit]
Description=todo-app service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}
EnvironmentFile=${ENV_FILE}
WorkingDirectory=/var/lib/todo-app
ExecStart=${APP_HOME}/bin/todo-app
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=/var/lib/todo-app

[Install]
WantedBy=multi-user.target
EOF

  chmod 0644 "$SERVICE_FILE"
  systemctl daemon-reload
}

install_release() {
  local tag="$1"
  local work_dir
  local release_dir
  validate_release_tag "$tag"
  work_dir="$(mktemp -d)"
  release_dir="$APP_HOME/releases/$tag"

  (
    trap 'rm -rf "$work_dir"' EXIT

    download_and_verify "$tag" "$work_dir"

    install -d -o root -g root -m 0755 "$release_dir"
    install -o root -g root -m 0755 "$work_dir/$RELEASE_ARTIFACT_NAME" "$release_dir/todo-app"
    ln -sfn "$release_dir/todo-app" "$APP_HOME/bin/todo-app"
    printf '%s\n' "$tag" >"$APP_HOME/current-release"
  )
}

main() {
  install_packages
  install_rds_ca_bundle
  install_cosign
  ensure_user

  local tag
  tag="${FALLBACK_RELEASE_TAG}"
  log "release tag: ${tag}"

  install_release "$tag"
  write_environment_file
  write_systemd_unit

  systemctl enable todo-app.service
  systemctl restart todo-app.service
  log "todo-app service restarted"
}

main "$@"
