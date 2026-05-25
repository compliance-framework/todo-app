# todo-app Terraform

This directory provisions the AWS environment for the SOC2/CCF todo-app demo. It defaults to `eu-west-2`; set `aws_region` to deploy in another region.

It creates a VPC with public ALB subnets and private EC2 subnets, VPC flow logs, a TLS ALB with access logs, and a size-1 Auto Scaling Group for the app host.

The app host enforces IMDSv2, uses an encrypted EBS root volume, and has a scoped instance profile for bootstrap parameter reads. The current Go app reads `PORT` and `DB_PATH`; bootstrap runs it with SQLite at `/var/lib/todo-app/todo_app.db`. The size-1 ASG is intentional: the demo Cloud Custodian policy flags bare EC2 instances.

By default the stack creates one NAT Gateway per AZ. Set `nat_gateway_mode = "single"` for lower-cost demo environments that can accept the single-AZ failure domain and cross-AZ egress charges.

## Apply

```bash
terraform init
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with the ACM certificate ARN, allowed HTTPS CIDRs,
# release tag, and pinned cosign checksums for the selected cosign_version.
terraform apply
```

Do not commit `terraform.tfvars`, state files, or secrets.

## Destroy

ALB access logs are retained in S3 for 90 days. Empty the ALB log bucket before running `terraform destroy`, otherwise Terraform cannot delete the non-empty bucket.

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

Terraform writes the desired release tag to SSM Parameter Store at `release_tag_parameter_name` and installs `terraform/scripts/bootstrap.sh` on the EC2 host. When `release_tag_parameter_name` is null, Terraform derives a stack-specific path of `/<name_prefix>-<environment>/release-tag`; override it when multiple stacks should intentionally share one release tag parameter. Bootstrap downloads `cosign`, verifies the binary against the pinned checksum supplied in Terraform variables, downloads the release artifact and sigstore bundle from GitHub, verifies the artifact with `cosign verify-blob`, installs the binary under `/opt/todo-app/releases/<tag>`, updates the `/opt/todo-app/bin/todo-app` symlink, writes `/etc/todo-app/todo-app.env` with `PORT` and `DB_PATH`, and restarts `todo-app.service`.

To upgrade, change `release_tag` in Terraform or update the SSM parameter directly, then rerun `/opt/todo-app/bootstrap.sh` on the instance. The stack creates an SSM Command document, exposed as `upgrade_ssm_document_name`, for that rerun. The existing release-triggered deployment workflow can later update the release tag parameter and invoke this document without changing the bootstrap contract.
