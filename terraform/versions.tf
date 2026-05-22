terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.80"
    }
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Application = "todo-app"
      Environment = var.environment
      ManagedBy   = "terraform"
      Ticket      = "BCH-1283"
    }
  }
}
