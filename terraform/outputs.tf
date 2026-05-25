output "alb_dns_name" {
  description = "ALB DNS name to target from the public domain."
  value       = aws_lb.app.dns_name
}

output "release_tag_parameter_name" {
  description = "SSM Parameter used by bootstrap.sh to select the installed release tag."
  value       = aws_ssm_parameter.release_tag.name
}

output "app_instance_role_name" {
  description = "IAM role attached to the app host instance profile."
  value       = aws_iam_role.app_instance.name
}

output "upgrade_ssm_document_name" {
  description = "SSM Command document that reruns bootstrap.sh for release upgrades."
  value       = aws_ssm_document.upgrade.name
}
