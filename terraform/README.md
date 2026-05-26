# todo-app Terraform

This directory provisions the AWS environment for the SOC2/CCF todo-app demo. It defaults to `eu-west-2`; set `aws_region` to deploy in another region.

It creates a VPC with public ALB subnets and private EC2 subnets, a TLS ALB, and a size-1 Auto Scaling Group for the app host. VPC flow logs and ALB access logs are optional and are created only when `enable_vpc_flow_logs` or `enable_alb_access_logs` are enabled.

The app host enforces IMDSv2, uses an encrypted EBS root volume, and has an instance profile with SSM Managed Instance Core plus scoped Secrets Manager `GetSecretValue` permission for the database password. Bootstrap configures the Go app to use the private RDS PostgreSQL instance with password authentication; the generated password is stored in Secrets Manager and fetched on the instance at runtime. The size-1 ASG is intentional: the demo Cloud Custodian policy flags bare EC2 instances.

By default the stack creates one NAT Gateway per AZ. Set `nat_gateway_mode = "single"` for lower-cost demo environments that can accept the single-AZ failure domain and cross-AZ egress charges.

## Apply

```bash
terraform init
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your domain, hosted zone, allowed HTTPS CIDRs,
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

Terraform writes bootstrap configuration into EC2 user data and installs `terraform/scripts/bootstrap.sh` on the EC2 host. Bootstrap downloads `cosign`, verifies the binary against the pinned checksum supplied in Terraform variables, downloads the release artifact and sigstore bundle from GitHub, verifies the artifact with `cosign verify-blob`, installs the binary under `/opt/todo-app/releases/<tag>`, updates the `/opt/todo-app/bin/todo-app` symlink, fetches the RDS password from Secrets Manager, writes `/etc/todo-app/todo-app.env` with `PORT` and the `DB_*` PostgreSQL settings, and restarts `todo-app.service`.

To upgrade through Terraform, change `release_tag` and apply the stack so replacement app instances receive the new value in user data as `FALLBACK_RELEASE_TAG` during bootstrap. For a manual rerun on an existing instance, execute `/opt/todo-app/bootstrap.sh`; it uses the bootstrap environment already written from user data, including the configured release tag and database secret ARN.
