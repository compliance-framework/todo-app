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
  description = "todo-app release tag baked into bootstrap.env as FALLBACK_RELEASE_TAG. Changes update the launch template for new or replaced instances; run an instance refresh, replacement, or ASG rollout to install a new release on existing capacity."
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

variable "oidc_issuer_url" {
  description = "OIDC provider issuer URL (e.g. https://accounts.google.com). Leave default for Google."
  type        = string
  default     = "https://accounts.google.com"
}

variable "oidc_client_id" {
  description = "OIDC client ID. Leave empty to disable OIDC login."
  type        = string
  default     = ""
}

variable "oidc_client_secret" {
  description = "OIDC client secret. Stored in Secrets Manager. Leave empty to disable OIDC login."
  type        = string
  default     = ""
  sensitive   = true
}

# ---------------------------------------------------------------------------
# CCF (Compliance Framework) — optional UI + API containers co-located on the
# same EC2 host as the todo-app, behind the same ALB on a dedicated hostname.
# All CCF resources are guarded by enable_ccf so the todo-app deploy is a no-op
# when CCF is disabled.
# ---------------------------------------------------------------------------

variable "enable_ccf" {
  description = "Run the CCF UI and API containers on the app host alongside the todo-app. When false, no CCF resources are created."
  type        = bool
  default     = false
}

variable "ccf_domain_name" {
  description = "Fully-qualified domain name for the CCF stack (e.g. ccf.ccfdemo.com). Must equal hosted_zone_name or be a subdomain of it. Added as a SAN on the ACM certificate and routed by the ALB to the CCF containers. Required when enable_ccf is true."
  type        = string
  default     = ""

  validation {
    condition     = var.ccf_domain_name == "" || can(regex("^([A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?\\.)+[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$", trimspace(var.ccf_domain_name)))
    error_message = "ccf_domain_name must be empty or a fully-qualified domain name such as ccf.example.com, without a trailing dot."
  }
}

variable "ccf_api_image" {
  description = "Container image for the CCF API, pulled on the host."
  type        = string
  default     = "ghcr.io/compliance-framework/api:0.16.2"
}

variable "ccf_ui_image" {
  description = "Container image for the CCF UI, pulled on the host."
  type        = string
  default     = "ghcr.io/compliance-framework/ui:2.9.2"
}

variable "ccf_api_host_port" {
  description = "Host port the CCF API container binds to. Must differ from app_port (used by the todo-app) and from ccf_ui_host_port."
  type        = number
  default     = 8081

  validation {
    condition     = var.ccf_api_host_port >= 1 && var.ccf_api_host_port <= 65535
    error_message = "ccf_api_host_port must be between 1 and 65535."
  }
}

variable "ccf_ui_host_port" {
  description = "Host port the CCF UI container binds to. Must differ from app_port (used by the todo-app) and from ccf_api_host_port."
  type        = number
  default     = 3000

  validation {
    condition     = var.ccf_ui_host_port >= 1 && var.ccf_ui_host_port <= 65535
    error_message = "ccf_ui_host_port must be between 1 and 65535."
  }
}

variable "ccf_db_name" {
  description = "Name of the PostgreSQL database created on the existing RDS instance for CCF. Created by the host bootstrap using the RDS master credentials."
  type        = string
  default     = "ccf"

  validation {
    condition     = can(regex("^[A-Za-z_][A-Za-z0-9_]*$", var.ccf_db_name))
    error_message = "ccf_db_name must be a valid PostgreSQL identifier (letters, digits, underscores; not starting with a digit)."
  }
}

# CCF SSO — Google-only, mirroring the local-dev sso.yaml. When the client id is
# set, the host renders an sso.yaml for the CCF API with Google login enabled.
variable "ccf_sso_google_client_id" {
  description = "Google OAuth client ID for CCF SSO. Leave empty to disable CCF SSO."
  type        = string
  default     = ""
}

variable "ccf_sso_google_client_secret" {
  description = "Google OAuth client secret for CCF SSO. Stored in Secrets Manager and fetched by the host at boot. Leave empty to disable CCF SSO."
  type        = string
  default     = ""
  sensitive   = true
}

variable "ccf_sso_google_hosted_domain" {
  description = "Google Workspace hosted domain (hd) whose users are mapped to ccf-authorized-users, matching the local-dev SSO group mapping."
  type        = string
  default     = "container-solutions.com"
}

variable "ccf_sso_admin_email" {
  description = "Email address mapped to ccf-admins in the CCF SSO group mapping. Leave empty to omit the email-based admin mapping."
  type        = string
  default     = ""
}

variable "ccf_sso_domain_admins" {
  description = "Grant admin (ccf-admins) to every user in ccf_sso_google_hosted_domain, in addition to ccf-authorized-users. Use with care: all domain users become CCF admins."
  type        = bool
  default     = false
}

# CCF agent (assessor) — runs on the same host as a container, collecting
# evidence via plugins and reporting to the local CCF API. AWS plugins use the
# instance role (read-only); GitHub/dependabot plugins use a PAT from Secrets
# Manager. Requires enable_ccf = true.
variable "enable_ccf_agent" {
  description = "Run the CCF agent (worker) container on the app host alongside the CCF UI/API. Requires enable_ccf = true."
  type        = bool
  default     = false
}

variable "ccf_agent_image" {
  description = "Container image for the CCF agent."
  type        = string
  default     = "ghcr.io/compliance-framework/agent:0.7.0"
}

variable "ccf_agent_github_token" {
  description = "GitHub PAT used by the agent's dependabot, github-settings, and github-repositories plugins. Stored in Secrets Manager and fetched at boot. Leave empty to run only the AWS plugins."
  type        = string
  default     = ""
  sensitive   = true
}
