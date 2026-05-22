# todo-app Terraform

This directory provisions the AWS environment for the SOC2/CCF todo-app demo in `eu-west-2`.

It creates a VPC with public ALB subnets and private EC2/RDS subnets, VPC flow logs, a TLS ALB with access logs, a size-1 Auto Scaling Group for the app host, and an encrypted RDS PostgreSQL instance with IAM database authentication, backups, deletion protection, Performance Insights, and enhanced monitoring.

The app host enforces IMDSv2, uses an encrypted EBS root volume, and has a scoped instance profile for `rds-db:connect`, bootstrap parameter reads, and CloudWatch log writes. The size-1 ASG is intentional: the demo Cloud Custodian policy flags bare EC2 instances.

## Apply

```bash
terraform init
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with the real domain, ACM certificate ARN, and release tag.
terraform apply
```

Do not commit `terraform.tfvars`, state files, or secrets.

## Validate Locally

In this directory:

```bash
terraform init -backend=false
terraform fmt -check -recursive
terraform validate
```

From the repo root, the app checks should still pass:

```bash
go build ./...
go test ./...
```

## Release Install And Upgrade

Terraform writes the desired release tag to SSM Parameter Store at `release_tag_parameter_name` and installs `terraform/scripts/bootstrap.sh` on the EC2 host. Bootstrap downloads the release artifact from GitHub, downloads the sigstore bundle, verifies it with `cosign verify-blob`, installs the binary under `/opt/todo-app/releases/<tag>`, updates the `/opt/todo-app/bin/todo-app` symlink, writes `/etc/todo-app/todo-app.env`, and restarts `todo-app.service`.

To upgrade, change `release_tag` in Terraform or update the SSM parameter directly, then rerun `/opt/todo-app/bootstrap.sh` on the instance. The stack creates an SSM Command document, exposed as `upgrade_ssm_document_name`, for that rerun. The existing release-triggered deployment workflow can later update the release tag parameter and invoke this document without changing the bootstrap contract.
