# Application secrets stored in Secrets Manager and fetched by the instance at boot.

resource "random_password" "jwt" {
  length  = 64
  special = false
}

resource "aws_secretsmanager_secret" "jwt" {
  name_prefix             = "${local.name}-jwt-"
  description             = "JWT signing secret for ${local.name}"
  recovery_window_in_days = 7
}

resource "aws_secretsmanager_secret_version" "jwt" {
  secret_id     = aws_secretsmanager_secret.jwt.id
  secret_string = random_password.jwt.result
}

resource "aws_secretsmanager_secret" "oidc_client_secret" {
  count = var.oidc_client_secret == "" ? 0 : 1

  name_prefix             = "${local.name}-oidc-client-secret-"
  description             = "OIDC client secret for ${local.name}"
  recovery_window_in_days = 7
}

resource "aws_secretsmanager_secret_version" "oidc_client_secret" {
  count = var.oidc_client_secret == "" ? 0 : 1

  secret_id     = aws_secretsmanager_secret.oidc_client_secret[0].id
  secret_string = var.oidc_client_secret
}
