# TODO: Replace password-based authentication with IAM database authentication.
# IAM auth removes the need to manage, rotate, or expose database passwords entirely.
# See: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html
resource "random_password" "db" {
  length  = 32
  special = false
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

  publicly_accessible     = false
  skip_final_snapshot     = true
  deletion_protection     = false
  backup_retention_period = 0

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
