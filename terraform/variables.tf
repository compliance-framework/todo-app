variable "aws_region" {
  description = "AWS region for the SOC2/CCF demo environment."
  type        = string
  default     = "eu-west-2"
}

variable "environment" {
  description = "Environment name used for tags and resource names."
  type        = string
  default     = "soc2-demo"
}

variable "name_prefix" {
  description = "Prefix for provisioned resource names."
  type        = string
  default     = "todo-app"
}

variable "domain_name" {
  description = "Public hostname that will be routed to the ALB."
  type        = string
}

variable "alb_certificate_arn" {
  description = "ACM certificate ARN for the HTTPS listener. The certificate must be in aws_region."
  type        = string
}

variable "allowed_https_cidr_blocks" {
  description = "CIDR blocks allowed to reach the ALB on HTTPS."
  type        = list(string)
  default     = ["0.0.0.0/0"]
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
  description = "CIDR blocks for private EC2 and RDS subnets."
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

variable "db_name" {
  description = "Application database name."
  type        = string
  default     = "todo_app"
}

variable "db_user" {
  description = "PostgreSQL application database user."
  type        = string
  default     = "todo_app"
}

variable "db_instance_class" {
  description = "RDS instance class."
  type        = string
  default     = "db.t4g.small"
}

variable "db_allocated_storage_gb" {
  description = "Initial RDS storage in GiB."
  type        = number
  default     = 20
}

variable "db_backup_retention_days" {
  description = "Automated backup retention period in days."
  type        = number
  default     = 7
}

variable "db_sslmode" {
  description = "PostgreSQL sslmode exported to the application EnvironmentFile."
  type        = string
  default     = "require"
}

variable "ec2_instance_type" {
  description = "EC2 instance type for the private app host."
  type        = string
  default     = "t3.micro"
}

variable "ec2_key_name" {
  description = "Optional EC2 key pair name for break-glass SSH access. Leave null for no key."
  type        = string
  default     = null
}

variable "jwt_secret_ssm_parameter_name" {
  description = "SSM Parameter name containing the JWT secret consumed by the backend."
  type        = string
  default     = "/todo-app/jwt-secret"
}

variable "oidc_issuer_url" {
  description = "OIDC issuer URL exported to the application EnvironmentFile."
  type        = string
  default     = ""
}

variable "oidc_client_id" {
  description = "OIDC client ID exported to the application EnvironmentFile."
  type        = string
  default     = ""
}

variable "oidc_client_secret_ssm_parameter_name" {
  description = "SSM Parameter name containing the OIDC client secret."
  type        = string
  default     = "/todo-app/oidc-client-secret"
}

variable "oidc_redirect_url" {
  description = "OIDC redirect URL exported to the application EnvironmentFile."
  type        = string
  default     = ""
}

variable "cors_allowed_origin" {
  description = "CORS allowed origin exported to the application EnvironmentFile."
  type        = string
  default     = ""
}
