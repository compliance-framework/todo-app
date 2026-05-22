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
    tags = merge(
      {
        Application = "todo-app"
        Environment = var.environment
        ManagedBy   = "terraform"
      },
      var.ticket_tag == null ? {} : { Ticket = var.ticket_tag }
    )
  }
}
