resource "random_password" "db" {
  length  = 32
  special = false
}

resource "aws_secretsmanager_secret" "db_password" {
  name_prefix             = "${local.name}-db-password-"
  description             = "Generated RDS password for ${local.name}"
  recovery_window_in_days = 7
}

resource "aws_secretsmanager_secret_version" "db_password" {
  secret_id     = aws_secretsmanager_secret.db_password.id
  secret_string = random_password.db.result
}

resource "aws_db_subnet_group" "app" {
  name       = local.name
  subnet_ids = aws_subnet.private[*].id
}

resource "aws_security_group" "rds" {
  name        = "${local.name}-rds"
  description = "Allow PostgreSQL only from app EC2 instances"
  vpc_id      = aws_vpc.app.id
}

resource "aws_vpc_security_group_ingress_rule" "rds_app" {
  security_group_id            = aws_security_group.rds.id
  description                  = "PostgreSQL from app instances"
  referenced_security_group_id = aws_security_group.app.id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
}

resource "aws_db_instance" "app" {
  identifier     = local.name
  engine         = "postgres"
  engine_version = "17"
  instance_class = var.db_instance_class

  allocated_storage = var.db_allocated_storage
  storage_encrypted = true
  storage_type      = "gp3"

  db_name  = var.db_name
  username = var.db_username
  password = random_password.db.result

  db_subnet_group_name   = aws_db_subnet_group.app.name
  vpc_security_group_ids = [aws_security_group.rds.id]

  publicly_accessible       = false
  skip_final_snapshot       = var.db_skip_final_snapshot
  final_snapshot_identifier = "${local.name}-final-snapshot"
  deletion_protection       = var.db_deletion_protection
  backup_retention_period   = var.db_backup_retention_period

  apply_immediately = true
}

output "rds_endpoint" {
  description = "RDS PostgreSQL endpoint (host:port)."
  value       = "${aws_db_instance.app.address}:${aws_db_instance.app.port}"
}

output "db_password" {
  description = "Auto-generated RDS master password. Store this securely after first apply."
  value       = random_password.db.result
  sensitive   = true
}

output "db_password_secret_arn" {
  description = "Secrets Manager secret ARN containing the generated RDS master password."
  value       = aws_secretsmanager_secret.db_password.arn
}
