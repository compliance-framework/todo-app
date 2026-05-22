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

variable "alb_certificate_arn" {
  description = "ACM certificate ARN for the HTTPS listener. The certificate must be in aws_region."
  type        = string
}

variable "allowed_https_cidr_blocks" {
  description = "CIDR blocks explicitly allowed to reach the ALB on HTTPS. Use [\"0.0.0.0/0\"] only when intentionally opening HTTPS to the public internet."
  type        = list(string)

  validation {
    condition     = length(var.allowed_https_cidr_blocks) > 0 && alltrue([for cidr in var.allowed_https_cidr_blocks : can(cidrhost(cidr, 0))])
    error_message = "allowed_https_cidr_blocks must contain at least one valid CIDR block."
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
}

variable "private_subnet_cidrs" {
  description = "CIDR blocks for private EC2 subnets."
  type        = list(string)
  default     = ["10.42.10.0/24", "10.42.11.0/24"]
}

variable "app_port" {
  description = "Port exposed by the Go backend and allowed from the ALB."
  type        = number
  default     = 8080
}

variable "release_tag" {
  description = "Fallback release tag to install when the SSM release tag parameter is unavailable."
  type        = string
  default     = "v0.1.0"
}

variable "release_tag_parameter_name" {
  description = "SSM Parameter name that stores the target release tag used by bootstrap upgrades."
  type        = string
  default     = "/todo-app/release-tag"
}

variable "github_repository" {
  description = "GitHub repository that publishes todo-app release artifacts, in owner/repo form."
  type        = string
  default     = "ContainerSolutions/todo-app"
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
  default     = "https://github.com/ContainerSolutions/todo-app/.github/workflows/.*"
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

variable "cosign_linux_amd64_sha256" {
  description = "Pinned SHA-256 checksum for the cosign Linux amd64 binary matching cosign_version."
  type        = string
}

variable "cosign_linux_arm64_sha256" {
  description = "Pinned SHA-256 checksum for the cosign Linux arm64 binary matching cosign_version."
  type        = string
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
