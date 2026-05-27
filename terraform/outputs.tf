output "alb_dns_name" {
  description = "ALB DNS name to target from the public domain."
  value       = aws_lb.app.dns_name
}

output "app_instance_role_name" {
  description = "IAM role attached to the app host instance profile."
  value       = aws_iam_role.app_instance.name
}
