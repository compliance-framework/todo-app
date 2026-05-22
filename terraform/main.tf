locals {
  availability_zone_count = min(length(var.public_subnet_cidrs), length(var.private_subnet_cidrs))
  name                    = "${var.name_prefix}-${var.environment}"
  alb_name                = substr(replace(local.name, "_", "-"), 0, 32)

  bootstrap_parameter_arns = [aws_ssm_parameter.release_tag.arn]
}

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

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
  map_public_ip_on_launch = true

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
  name              = "/aws/vpc/${local.name}/flow-logs"
  retention_in_days = 30
}

resource "aws_iam_role" "vpc_flow_logs" {
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
  name = "${local.name}-vpc-flow-logs"
  role = aws_iam_role.vpc_flow_logs.id

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
        Resource = "${aws_cloudwatch_log_group.vpc_flow_logs.arn}:*"
      }
    ]
  })
}

resource "aws_flow_log" "app" {
  vpc_id               = aws_vpc.app.id
  traffic_type         = "ALL"
  log_destination      = aws_cloudwatch_log_group.vpc_flow_logs.arn
  iam_role_arn         = aws_iam_role.vpc_flow_logs.arn
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

resource "aws_security_group_rule" "alb_ingress_https" {
  type              = "ingress"
  description       = "HTTPS"
  security_group_id = aws_security_group.alb.id
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = var.allowed_https_cidr_blocks
}

resource "aws_security_group_rule" "alb_egress_app" {
  type                     = "egress"
  description              = "App traffic to EC2"
  security_group_id        = aws_security_group.alb.id
  from_port                = var.app_port
  to_port                  = var.app_port
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.app.id
}

resource "aws_security_group_rule" "app_ingress_alb" {
  type                     = "ingress"
  description              = "App port from ALB"
  security_group_id        = aws_security_group.app.id
  from_port                = var.app_port
  to_port                  = var.app_port
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.alb.id
}

resource "aws_security_group_rule" "app_egress_all" {
  type              = "egress"
  description       = "Outbound for release downloads and SSM"
  security_group_id = aws_security_group.app.id
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_s3_bucket" "alb_logs" {
  bucket_prefix = "${local.name}-alb-logs-"
}

resource "aws_s3_bucket_public_access_block" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  rule {
    id     = "expire-alb-logs"
    status = "Enabled"

    expiration {
      days = 90
    }
  }
}

resource "aws_s3_bucket_policy" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id
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
        Resource = aws_s3_bucket.alb_logs.arn
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
        Resource = "${aws_s3_bucket.alb_logs.arn}/alb/AWSLogs/${data.aws_caller_identity.current.account_id}/*"
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
}

resource "aws_lb" "app" {
  name               = local.alb_name
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id

  access_logs {
    bucket  = aws_s3_bucket.alb_logs.id
    prefix  = "alb"
    enabled = true
  }

  depends_on = [aws_s3_bucket_policy.alb_logs]
}

resource "aws_lb_target_group" "app" {
  name     = substr("${replace(local.name, "_", "-")}-app", 0, 32)
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
  certificate_arn   = var.alb_certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app.arn
  }
}

resource "aws_ssm_parameter" "release_tag" {
  name        = var.release_tag_parameter_name
  description = "Target todo-app release tag installed by bootstrap.sh"
  type        = "String"
  value       = var.release_tag
  overwrite   = true
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

resource "aws_iam_role_policy" "app_instance" {
  name = "${local.name}-ec2"
  role = aws_iam_role.app_instance.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "ReadBootstrapParameters"
        Effect = "Allow"
        Action = [
          "ssm:GetParameter"
        ]
        Resource = local.bootstrap_parameter_arns
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "app_instance_ssm" {
  role       = aws_iam_role.app_instance.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "app" {
  name = "${local.name}-ec2"
  role = aws_iam_role.app_instance.name
}

resource "aws_ssm_document" "upgrade" {
  name          = "${local.name}-upgrade"
  document_type = "Command"

  content = jsonencode({
    schemaVersion = "2.2"
    description   = "Rerun todo-app bootstrap.sh to install the release tag from SSM Parameter Store."
    mainSteps = [
      {
        action = "aws:runShellScript"
        name   = "runBootstrap"
        inputs = {
          runCommand = [
            "sudo /opt/todo-app/bootstrap.sh"
          ]
        }
      }
    ]
  })
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
      volume_size = 20
      volume_type = "gp3"
    }
  }

  network_interfaces {
    associate_public_ip_address = false
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
    release_tag_parameter_name         = var.release_tag_parameter_name
  }))

  tag_specifications {
    resource_type = "instance"

    tags = {
      Name = local.name
    }
  }
}

resource "aws_autoscaling_group" "app" {
  name                = local.name
  min_size            = 1
  max_size            = 1
  desired_capacity    = 1
  vpc_zone_identifier = aws_subnet.private[*].id
  health_check_type   = "ELB"

  launch_template {
    id      = aws_launch_template.app.id
    version = "$Latest"
  }

  instance_refresh {
    strategy = "Rolling"

    preferences {
      min_healthy_percentage = 0
    }
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
}

resource "aws_autoscaling_attachment" "app" {
  autoscaling_group_name = aws_autoscaling_group.app.id
  lb_target_group_arn    = aws_lb_target_group.app.arn
}
