locals {
  availability_zone_count = min(length(var.public_subnet_cidrs), length(var.private_subnet_cidrs), length(data.aws_availability_zones.available.names))
  name                    = "${var.name_prefix}-${var.environment}"
  alb_name                = substr(replace(local.name, "_", "-"), 0, 32)
  alb_logs_bucket_prefix  = "${substr(local.name, 0, 27)}-alb-logs-"
  domain_name_normalized  = trimprefix(trimsuffix(lower(var.domain_name), "."), "*.")
  hosted_zone_normalized  = trimsuffix(lower(var.hosted_zone_name), ".")
  target_group_name       = trimsuffix(substr("${replace(local.name, "_", "-")}-app", 0, 32), "-")
  vpc_flow_log_group_name = "/aws/vpc/${local.name}/flow-logs"
  vpc_flow_log_group_arn  = "arn:aws:logs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:log-group:${local.vpc_flow_log_group_name}"
}

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

resource "terraform_data" "input_validation" {
  input = true

  lifecycle {
    precondition {
      condition     = length(local.name) <= 32
      error_message = "name_prefix plus environment must be 32 characters or fewer when combined as name_prefix-environment for derived AWS resource names."
    }

    precondition {
      condition     = length(var.public_subnet_cidrs) == length(var.private_subnet_cidrs)
      error_message = "public_subnet_cidrs and private_subnet_cidrs must contain the same number of CIDR blocks."
    }

    precondition {
      condition     = length(var.public_subnet_cidrs) <= length(data.aws_availability_zones.available.names)
      error_message = "The number of public/private subnet CIDR blocks cannot exceed the number of available availability zones in the selected region."
    }

    precondition {
      condition = (
        local.domain_name_normalized == local.hosted_zone_normalized ||
        endswith(local.domain_name_normalized, ".${local.hosted_zone_normalized}")
      )
      error_message = "domain_name must be equal to hosted_zone_name or a subdomain of it so ACM DNS validation records are created in the correct Route53 zone."
    }

    precondition {
      condition = (
        (var.ec2_ami_architecture == "x86_64" && can(regex("linux-amd64", var.release_artifact_name))) ||
        (var.ec2_ami_architecture == "arm64" && can(regex("linux-arm64", var.release_artifact_name)))
      )
      error_message = "release_artifact_name must match ec2_ami_architecture: use a linux-amd64 artifact with x86_64 and a linux-arm64 artifact with arm64."
    }

    precondition {
      condition = var.skip_cosign_verify || (
        (var.ec2_ami_architecture == "x86_64" && can(regex("linux-amd64\\.bundle$", var.release_signature_bundle_name))) ||
        (var.ec2_ami_architecture == "arm64" && can(regex("linux-arm64\\.bundle$", var.release_signature_bundle_name)))
      )
      error_message = "release_signature_bundle_name must match ec2_ami_architecture and end in .bundle: use a linux-amd64 bundle with x86_64 and a linux-arm64 bundle with arm64."
    }
  }
}

data "aws_ami" "amazon_linux" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-${var.ec2_ami_architecture}"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_vpc" "app" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = local.name
  }
}

resource "aws_internet_gateway" "app" {
  vpc_id = aws_vpc.app.id

  tags = {
    Name = local.name
  }
}

resource "aws_subnet" "public" {
  count = local.availability_zone_count

  vpc_id                  = aws_vpc.app.id
  cidr_block              = var.public_subnet_cidrs[count.index]
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = false

  tags = {
    Name = "${local.name}-public-${count.index + 1}"
  }
}

resource "aws_subnet" "private" {
  count = local.availability_zone_count

  vpc_id            = aws_vpc.app.id
  cidr_block        = var.private_subnet_cidrs[count.index]
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = {
    Name = "${local.name}-private-${count.index + 1}"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.app.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.app.id
  }

  tags = {
    Name = "${local.name}-public"
  }
}

resource "aws_route_table_association" "public" {
  count = length(aws_subnet.public)

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_eip" "nat" {
  count = var.nat_gateway_mode == "single" ? 1 : length(aws_subnet.public)

  domain = "vpc"

  depends_on = [aws_internet_gateway.app]

  tags = {
    Name = "${local.name}-nat-${count.index + 1}"
  }
}

resource "aws_nat_gateway" "app" {
  count = var.nat_gateway_mode == "single" ? 1 : length(aws_subnet.public)

  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = aws_subnet.public[var.nat_gateway_mode == "single" ? 0 : count.index].id

  tags = {
    Name = "${local.name}-nat-${count.index + 1}"
  }
}

resource "aws_route_table" "private" {
  count = length(aws_subnet.private)

  vpc_id = aws_vpc.app.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.app[var.nat_gateway_mode == "single" ? 0 : count.index].id
  }

  tags = {
    Name = "${local.name}-private-${count.index + 1}"
  }
}

resource "aws_route_table_association" "private" {
  count = length(aws_subnet.private)

  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private[count.index].id
}

resource "aws_cloudwatch_log_group" "vpc_flow_logs" {
  count = var.enable_vpc_flow_logs ? 1 : 0

  name              = local.vpc_flow_log_group_name
  kms_key_id        = aws_kms_key.vpc_flow_logs[0].arn
  retention_in_days = 30
}

resource "aws_kms_key" "vpc_flow_logs" {
  count = var.enable_vpc_flow_logs ? 1 : 0

  description         = "KMS key for ${local.name} VPC flow logs"
  enable_key_rotation = true

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "EnableAccountKeyAdministration"
        Effect = "Allow"
        Principal = {
          AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"
        }
        Action   = "kms:*"
        Resource = "*"
      },
      {
        Sid    = "AllowCloudWatchLogsUse"
        Effect = "Allow"
        Principal = {
          Service = "logs.${var.aws_region}.amazonaws.com"
        }
        Action = [
          "kms:Encrypt",
          "kms:Decrypt",
          "kms:ReEncrypt*",
          "kms:GenerateDataKey*",
          "kms:DescribeKey"
        ]
        Resource = "*"
        Condition = {
          ArnEquals = {
            "kms:EncryptionContext:aws:logs:arn" = local.vpc_flow_log_group_arn
          }
        }
      }
    ]
  })
}

resource "aws_kms_alias" "vpc_flow_logs" {
  count = var.enable_vpc_flow_logs ? 1 : 0

  name          = "alias/${local.name}-vpc-flow-logs"
  target_key_id = aws_kms_key.vpc_flow_logs[0].key_id
}

resource "aws_iam_role" "vpc_flow_logs" {
  count = var.enable_vpc_flow_logs ? 1 : 0

  name = "${local.name}-vpc-flow-logs"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "vpc-flow-logs.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}

resource "aws_iam_role_policy" "vpc_flow_logs" {
  count = var.enable_vpc_flow_logs ? 1 : 0

  name = "${local.name}-vpc-flow-logs"
  role = aws_iam_role.vpc_flow_logs[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:DescribeLogGroups",
          "logs:DescribeLogStreams"
        ]
        Resource = "${aws_cloudwatch_log_group.vpc_flow_logs[0].arn}:*"
      }
    ]
  })
}

resource "aws_flow_log" "app" {
  count = var.enable_vpc_flow_logs ? 1 : 0

  vpc_id               = aws_vpc.app.id
  traffic_type         = "ALL"
  log_destination      = aws_cloudwatch_log_group.vpc_flow_logs[0].arn
  iam_role_arn         = aws_iam_role.vpc_flow_logs[0].arn
  log_destination_type = "cloud-watch-logs"
}

resource "aws_security_group" "alb" {
  name        = "${local.name}-alb"
  description = "Allow HTTPS from the internet to the ALB"
  vpc_id      = aws_vpc.app.id
}

resource "aws_security_group" "app" {
  name        = "${local.name}-app"
  description = "Allow app traffic only from the ALB"
  vpc_id      = aws_vpc.app.id
}

resource "aws_vpc_security_group_ingress_rule" "alb_https" {
  for_each = toset(var.allowed_https_cidr_blocks)

  security_group_id = aws_security_group.alb.id
  description       = "HTTPS"
  cidr_ipv4         = each.value
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "alb_app" {
  security_group_id            = aws_security_group.alb.id
  description                  = "App traffic to EC2"
  referenced_security_group_id = aws_security_group.app.id
  from_port                    = var.app_port
  to_port                      = var.app_port
  ip_protocol                  = "tcp"
}

resource "aws_vpc_security_group_ingress_rule" "app_alb" {
  security_group_id            = aws_security_group.app.id
  description                  = "App port from ALB"
  referenced_security_group_id = aws_security_group.alb.id
  from_port                    = var.app_port
  to_port                      = var.app_port
  ip_protocol                  = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "app_https" {
  security_group_id = aws_security_group.app.id
  description       = "Outbound for release downloads and SSM"
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "app_dns_udp" {
  security_group_id = aws_security_group.app.id
  description       = "DNS to the VPC resolver"
  cidr_ipv4         = "169.254.169.253/32"
  from_port         = 53
  to_port           = 53
  ip_protocol       = "udp"
}

resource "aws_vpc_security_group_egress_rule" "app_dns_tcp" {
  security_group_id = aws_security_group.app.id
  description       = "DNS to the VPC resolver"
  cidr_ipv4         = "169.254.169.253/32"
  from_port         = 53
  to_port           = 53
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "app_rds" {
  security_group_id            = aws_security_group.app.id
  description                  = "PostgreSQL to RDS"
  referenced_security_group_id = aws_security_group.rds.id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
}

resource "aws_s3_bucket" "alb_logs" {
  count = var.enable_alb_access_logs ? 1 : 0

  bucket_prefix = local.alb_logs_bucket_prefix
}

resource "aws_s3_bucket_ownership_controls" "alb_logs" {
  count = var.enable_alb_access_logs ? 1 : 0

  bucket = aws_s3_bucket.alb_logs[0].id

  rule {
    object_ownership = "BucketOwnerPreferred"
  }
}

resource "aws_s3_bucket_acl" "alb_logs" {
  count = var.enable_alb_access_logs ? 1 : 0

  bucket = aws_s3_bucket.alb_logs[0].id
  acl    = "private"

  depends_on = [aws_s3_bucket_ownership_controls.alb_logs]
}

resource "aws_s3_bucket_public_access_block" "alb_logs" {
  count = var.enable_alb_access_logs ? 1 : 0

  bucket = aws_s3_bucket.alb_logs[0].id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "alb_logs" {
  count = var.enable_alb_access_logs ? 1 : 0

  bucket = aws_s3_bucket.alb_logs[0].id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "alb_logs" {
  count = var.enable_alb_access_logs ? 1 : 0

  bucket = aws_s3_bucket.alb_logs[0].id

  rule {
    id     = "expire-alb-logs"
    status = "Enabled"

    filter {}

    expiration {
      days = 90
    }
  }
}

resource "aws_s3_bucket_policy" "alb_logs" {
  count = var.enable_alb_access_logs ? 1 : 0

  bucket = aws_s3_bucket.alb_logs[0].id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AWSALBLogDeliveryAclCheck"
        Effect = "Allow"
        Principal = {
          Service = "logdelivery.elasticloadbalancing.amazonaws.com"
        }
        Action   = "s3:GetBucketAcl"
        Resource = aws_s3_bucket.alb_logs[0].arn
        Condition = {
          StringEquals = {
            "aws:SourceAccount" = data.aws_caller_identity.current.account_id
          }
          ArnLike = {
            "aws:SourceArn" = "arn:aws:elasticloadbalancing:${var.aws_region}:${data.aws_caller_identity.current.account_id}:loadbalancer/app/${local.alb_name}/*"
          }
        }
      },
      {
        Sid    = "AWSALBLogDeliveryWrite"
        Effect = "Allow"
        Principal = {
          Service = "logdelivery.elasticloadbalancing.amazonaws.com"
        }
        Action   = "s3:PutObject"
        Resource = "${aws_s3_bucket.alb_logs[0].arn}/alb/AWSLogs/${data.aws_caller_identity.current.account_id}/*"
        Condition = {
          StringEquals = {
            "aws:SourceAccount" = data.aws_caller_identity.current.account_id
            "s3:x-amz-acl"      = "bucket-owner-full-control"
          }
          ArnLike = {
            "aws:SourceArn" = "arn:aws:elasticloadbalancing:${var.aws_region}:${data.aws_caller_identity.current.account_id}:loadbalancer/app/${local.alb_name}/*"
          }
        }
      }
    ]
  })

  depends_on = [aws_s3_bucket_acl.alb_logs]
}

resource "aws_lb" "app" {
  name                       = local.alb_name
  internal                   = false
  load_balancer_type         = "application"
  drop_invalid_header_fields = true
  security_groups            = [aws_security_group.alb.id]
  subnets                    = aws_subnet.public[*].id

  dynamic "access_logs" {
    for_each = var.enable_alb_access_logs ? [1] : []
    content {
      bucket  = aws_s3_bucket.alb_logs[0].id
      prefix  = "alb"
      enabled = true
    }
  }

  depends_on = [aws_s3_bucket_policy.alb_logs]
}

resource "aws_lb_target_group" "app" {
  name     = local.target_group_name
  port     = var.app_port
  protocol = "HTTP"
  vpc_id   = aws_vpc.app.id

  health_check {
    enabled             = true
    healthy_threshold   = 2
    interval            = 30
    matcher             = "200"
    path                = "/health"
    port                = "traffic-port"
    protocol            = "HTTP"
    timeout             = 5
    unhealthy_threshold = 3
  }
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.app.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = aws_acm_certificate_validation.app.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app.arn
  }
}

resource "aws_iam_role" "app_instance" {
  name = "${local.name}-ec2"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "app_instance_ssm" {
  role       = aws_iam_role.app_instance.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_role_policy" "app_instance_db_password" {
  name = "${local.name}-db-password"
  role = aws_iam_role.app_instance.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = "secretsmanager:GetSecretValue"
        Resource = aws_secretsmanager_secret.db_password.arn
      }
    ]
  })
}

resource "aws_iam_instance_profile" "app" {
  provider = aws.no_default_tags
  name     = "${local.name}-ec2"
  role     = aws_iam_role.app_instance.name
}

resource "aws_launch_template" "app" {
  name_prefix   = "${local.name}-"
  image_id      = data.aws_ami.amazon_linux.id
  instance_type = var.ec2_instance_type
  key_name      = var.ec2_key_name

  iam_instance_profile {
    name = aws_iam_instance_profile.app.name
  }

  metadata_options {
    http_endpoint = "enabled"
    http_tokens   = "required"
  }

  block_device_mappings {
    device_name = "/dev/xvda"

    ebs {
      encrypted   = true
      volume_size = 30
      volume_type = "gp3"
    }
  }

  network_interfaces {
    associate_public_ip_address = false
    device_index                = 0
    security_groups             = [aws_security_group.app.id]
  }

  user_data = base64encode(templatefile("${path.module}/templates/user_data.sh.tftpl", {
    app_port                           = var.app_port
    aws_region                         = var.aws_region
    bootstrap_script                   = file("${path.module}/scripts/bootstrap.sh")
    cosign_certificate_identity_regexp = var.cosign_certificate_identity_regexp
    cosign_certificate_oidc_issuer     = var.cosign_certificate_oidc_issuer
    cosign_linux_amd64_sha256          = var.cosign_linux_amd64_sha256
    cosign_linux_arm64_sha256          = var.cosign_linux_arm64_sha256
    cosign_version                     = var.cosign_version
    fallback_release_tag               = var.release_tag
    github_repository                  = var.github_repository
    release_artifact_name              = var.release_artifact_name
    release_signature_bundle_name      = var.release_signature_bundle_name
    skip_cosign_verify                 = var.skip_cosign_verify
    db_host                            = aws_db_instance.app.address
    db_port                            = aws_db_instance.app.port
    db_name                            = var.db_name
    db_user                            = var.db_username
    db_password_secret_arn             = aws_secretsmanager_secret.db_password.arn
  }))

  depends_on = [aws_secretsmanager_secret_version.db_password]

  tag_specifications {
    resource_type = "instance"

    tags = merge(
      {
        Name        = local.name
        Application = "todo-app"
        Environment = var.environment
        ManagedBy   = "terraform"
      },
      var.ticket_tag == null ? {} : { Ticket = var.ticket_tag }
    )
  }

  tag_specifications {
    resource_type = "volume"

    tags = merge(
      {
        Name        = "${local.name}-volume"
        Application = "todo-app"
        Environment = var.environment
        ManagedBy   = "terraform"
      },
      var.ticket_tag == null ? {} : { Ticket = var.ticket_tag }
    )
  }
}

resource "aws_autoscaling_group" "app" {
  name                      = local.name
  min_size                  = 1
  max_size                  = 1
  desired_capacity          = 1
  vpc_zone_identifier       = aws_subnet.private[*].id
  health_check_type         = "ELB"
  health_check_grace_period = 600

  launch_template {
    id      = aws_launch_template.app.id
    version = "$Latest"
  }

  tag {
    key                 = "Name"
    value               = local.name
    propagate_at_launch = true
  }

  tag {
    key                 = "cloud-custodian-note"
    value               = "Size-1 ASG used so the demo EC2 host is not a bare instance."
    propagate_at_launch = true
  }

  dynamic "tag" {
    for_each = merge(
      {
        Application = "todo-app"
        Environment = var.environment
        ManagedBy   = "terraform"
      },
      var.ticket_tag == null ? {} : { Ticket = var.ticket_tag }
    )

    content {
      key                 = tag.key
      value               = tag.value
      propagate_at_launch = true
    }
  }
}

resource "aws_autoscaling_attachment" "app" {
  autoscaling_group_name = aws_autoscaling_group.app.id
  lb_target_group_arn    = aws_lb_target_group.app.arn
}
