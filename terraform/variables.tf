variable "aws_region" {
  description = "AWS region for the SOC2/CCF demo environment."
  type        = string
  default     = "eu-west-2"
}

variable "environment" {
  description = "Environment name used for tags and resource names."
  type        = string
  default     = "soc2-demo"

  validation {
    condition     = can(regex("^([a-z0-9]|[a-z0-9][a-z0-9-]*[a-z0-9])$", var.environment))
    error_message = "environment must contain only lowercase letters, numbers, and hyphens, and must not start or end with a hyphen."
  }
}

variable "name_prefix" {
  description = "Prefix for provisioned resource names."
  type        = string
  default     = "todo-app"

  validation {
    condition     = can(regex("^([a-z0-9]|[a-z0-9][a-z0-9-]*[a-z0-9])$", var.name_prefix))
    error_message = "name_prefix must contain only lowercase letters, numbers, and hyphens, and must not start or end with a hyphen."
  }
}

variable "domain_name" {
  description = "Fully-qualified domain name for the application (e.g. todo.ccfdemo.com). Used for the ACM certificate and ALB HTTPS listener."
  type        = string

  validation {
    condition     = can(regex("^([A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?\\.)+[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$", trimspace(var.domain_name)))
    error_message = "domain_name must be a non-empty fully-qualified domain name such as todo.example.com, without a trailing dot."
  }
}

variable "hosted_zone_name" {
  description = "Name of the existing Route53 public hosted zone that contains domain_name (e.g. ccfdemo.com). Terraform writes DNS validation records into this zone."
  type        = string

  validation {
    condition     = can(regex("^([A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?\\.)+[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$", trimsuffix(trimspace(var.hosted_zone_name), ".")))
    error_message = "hosted_zone_name must be a non-empty DNS zone name such as example.com, with an optional trailing dot."
  }
}

variable "enable_vpc_flow_logs" {
  description = "Enable VPC flow logs to CloudWatch Logs (with a dedicated KMS key and IAM role). Disabled by default to reduce cost."
  type        = bool
  default     = false
}

variable "enable_alb_access_logs" {
  description = "Enable ALB access logs to an S3 bucket. Disabled by default to reduce cost."
  type        = bool
  default     = false
}

variable "allowed_https_cidr_blocks" {
  description = "CIDR blocks explicitly allowed to reach the ALB on HTTPS. Use [\"0.0.0.0/0\"] only when intentionally opening HTTPS to the public internet."
  type        = list(string)

  validation {
    condition = length(var.allowed_https_cidr_blocks) > 0 && alltrue([
      for cidr in var.allowed_https_cidr_blocks :
      can(cidrhost(cidr, 0)) && can(regex("^([0-9]{1,3}\\.){3}[0-9]{1,3}/([0-9]|[12][0-9]|3[0-2])$", cidr))
    ])
    error_message = "allowed_https_cidr_blocks must contain at least one valid IPv4 CIDR block."
  }
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC."
  type        = string
  default     = "10.42.0.0/16"
}

variable "public_subnet_cidrs" {
  description = "CIDR blocks for public ALB subnets."
  type        = list(string)
  default     = ["10.42.0.0/24", "10.42.1.0/24"]

  validation {
    condition     = length(var.public_subnet_cidrs) >= 2
    error_message = "public_subnet_cidrs must contain at least two CIDR blocks for the ALB."
  }

  validation {
    condition     = alltrue([for cidr in var.public_subnet_cidrs : can(cidrhost(cidr, 0))])
    error_message = "public_subnet_cidrs must contain only valid CIDR blocks."
  }
}

variable "private_subnet_cidrs" {
  description = "CIDR blocks for private EC2 subnets."
  type        = list(string)
  default     = ["10.42.10.0/24", "10.42.11.0/24"]

  validation {
    condition     = length(var.private_subnet_cidrs) >= 2
    error_message = "private_subnet_cidrs must contain at least two CIDR blocks."
  }

  validation {
    condition     = alltrue([for cidr in var.private_subnet_cidrs : can(cidrhost(cidr, 0))])
    error_message = "private_subnet_cidrs must contain only valid CIDR blocks."
  }
}

variable "app_port" {
  description = "Port exposed by the Go backend and allowed from the ALB."
  type        = number
  default     = 8080

  validation {
    condition     = var.app_port >= 1 && var.app_port <= 65535
    error_message = "app_port must be between 1 and 65535."
  }
}

variable "release_tag" {
  description = "todo-app release tag baked into bootstrap.env as FALLBACK_RELEASE_TAG. Change and re-apply to deploy a new version."
  type        = string
  default     = "v0.1.0"
}

variable "github_repository" {
  description = "GitHub repository that publishes todo-app release artifacts, in owner/repo form."
  type        = string
  default     = "compliance-framework/todo-app"
}

variable "release_artifact_name" {
  description = "Release asset name for the Linux binary."
  type        = string
  default     = "todo-app-linux-amd64"
}

variable "release_signature_bundle_name" {
  description = "Release asset name for the sigstore bundle used by cosign verify-blob."
  type        = string
  default     = "todo-app-linux-amd64.bundle"
}

variable "cosign_certificate_identity_regexp" {
  description = "Expected signing identity regexp for cosign keyless verification."
  type        = string
  default     = "https://github.com/compliance-framework/todo-app/.github/workflows/.*"
}

variable "cosign_certificate_oidc_issuer" {
  description = "Expected OIDC issuer for cosign keyless verification."
  type        = string
  default     = "https://token.actions.githubusercontent.com"
}

variable "cosign_version" {
  description = "Cosign version installed by the bootstrap script."
  type        = string
  default     = "v2.4.3"
}

variable "skip_cosign_verify" {
  description = "Skip cosign signature verification during bootstrap. Signature verification is enabled by default; set to true only when release assets do not include a sigstore bundle."
  type        = bool
  default     = false
}

variable "cosign_linux_amd64_sha256" {
  description = "Pinned SHA-256 checksum for the cosign Linux amd64 binary matching cosign_version."
  type        = string

  validation {
    condition     = can(regex("^[0-9a-fA-F]{64}$", var.cosign_linux_amd64_sha256))
    error_message = "cosign_linux_amd64_sha256 must be a 64-character hexadecimal SHA-256 digest."
  }
}

variable "cosign_linux_arm64_sha256" {
  description = "Pinned SHA-256 checksum for the cosign Linux arm64 binary matching cosign_version."
  type        = string

  validation {
    condition     = can(regex("^[0-9a-fA-F]{64}$", var.cosign_linux_arm64_sha256))
    error_message = "cosign_linux_arm64_sha256 must be a 64-character hexadecimal SHA-256 digest."
  }
}

variable "nat_gateway_mode" {
  description = "NAT Gateway placement mode. Use per_az for high availability or single for lower-cost demo environments."
  type        = string
  default     = "per_az"

  validation {
    condition     = contains(["per_az", "single"], var.nat_gateway_mode)
    error_message = "nat_gateway_mode must be either per_az or single."
  }
}

variable "ec2_instance_type" {
  description = "EC2 instance type for the private app host."
  type        = string
  default     = "t3.micro"
}

variable "ec2_ami_architecture" {
  description = "Amazon Linux 2023 AMI architecture. Use arm64 for ARM instance families such as t4g; use x86_64 for t3/t4i/m families."
  type        = string
  default     = "x86_64"

  validation {
    condition     = contains(["x86_64", "arm64"], var.ec2_ami_architecture)
    error_message = "ec2_ami_architecture must be either x86_64 or arm64."
  }
}

variable "ec2_key_name" {
  description = "Optional EC2 key pair name for break-glass SSH access. Leave null for no key."
  type        = string
  default     = null
}

variable "ticket_tag" {
  description = "Optional Ticket tag value applied to all resources. Leave null to omit the Ticket tag."
  type        = string
  default     = null
}

variable "db_instance_class" {
  description = "RDS instance class."
  type        = string
  default     = "db.t3.micro"
}

variable "db_allocated_storage" {
  description = "Allocated storage for the RDS instance in GiB."
  type        = number
  default     = 20
}

variable "db_backup_retention_period" {
  description = "Number of days to retain automated RDS backups. Set to 0 only for disposable demo environments."
  type        = number
  default     = 7

  validation {
    condition     = var.db_backup_retention_period >= 0 && var.db_backup_retention_period <= 35
    error_message = "db_backup_retention_period must be between 0 and 35 days."
  }
}

variable "db_skip_final_snapshot" {
  description = "Skip the final RDS snapshot on destroy. Set true only for disposable demo environments."
  type        = bool
  default     = false
}

variable "db_deletion_protection" {
  description = "Enable RDS deletion protection."
  type        = bool
  default     = true
}

variable "db_name" {
  description = "Name of the PostgreSQL database created on the RDS instance."
  type        = string
  default     = "tododb"
}

variable "db_username" {
  description = "Master username for the RDS PostgreSQL instance."
  type        = string
  default     = "todoapp"
}
