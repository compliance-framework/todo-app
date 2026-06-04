output "alb_dns_name" {
  description = "ALB DNS name to target from the public domain."
  value       = aws_lb.app.dns_name
}

output "app_instance_role_name" {
  description = "IAM role attached to the app host instance profile."
  value       = aws_iam_role.app_instance.name
}

output "ccf_url" {
  description = "Public URL for the CCF UI when enable_ccf is true; empty otherwise."
  value       = var.enable_ccf ? "https://${var.ccf_domain_name}" : ""
}
