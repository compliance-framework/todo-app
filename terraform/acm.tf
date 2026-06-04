# ACM certificate — Terraform creates the certificate, DNS validation record,
# and waits for validation. Only the Route53 hosted zone must pre-exist.

data "aws_route53_zone" "app" {
  name         = var.hosted_zone_name
  private_zone = false
}

resource "aws_acm_certificate" "app" {
  domain_name       = var.domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_route53_record" "cert_validation" {
  for_each = {
    for dvo in aws_acm_certificate.app.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  }

  allow_overwrite = true
  name            = each.value.name
  records         = [each.value.record]
  ttl             = 60
  type            = each.value.type
  zone_id         = data.aws_route53_zone.app.zone_id
}

resource "aws_acm_certificate_validation" "app" {
  certificate_arn         = aws_acm_certificate.app.arn
  validation_record_fqdns = [for record in aws_route53_record.cert_validation : record.fqdn]
}

resource "aws_route53_record" "app" {
  zone_id = data.aws_route53_zone.app.zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = aws_lb.app.dns_name
    zone_id                = aws_lb.app.zone_id
    evaluate_target_health = true
  }
}

resource "aws_route53_record" "ccf" {
  count = var.enable_ccf ? 1 : 0

  zone_id = data.aws_route53_zone.app.zone_id
  name    = var.ccf_domain_name
  type    = "A"

  alias {
    name                   = aws_lb.app.dns_name
    zone_id                = aws_lb.app.zone_id
    evaluate_target_health = true
  }
}

# Separate certificate for the CCF hostname, attached to the same HTTPS listener
# via SNI. Kept independent of the todo-app certificate so enabling CCF never
# reissues or disrupts the existing todo.* cert.
resource "aws_acm_certificate" "ccf" {
  count = var.enable_ccf ? 1 : 0

  domain_name       = var.ccf_domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_route53_record" "ccf_cert_validation" {
  for_each = var.enable_ccf ? {
    for dvo in aws_acm_certificate.ccf[0].domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  } : {}

  allow_overwrite = true
  name            = each.value.name
  records         = [each.value.record]
  ttl             = 60
  type            = each.value.type
  zone_id         = data.aws_route53_zone.app.zone_id
}

resource "aws_acm_certificate_validation" "ccf" {
  count = var.enable_ccf ? 1 : 0

  certificate_arn         = aws_acm_certificate.ccf[0].arn
  validation_record_fqdns = [for record in aws_route53_record.ccf_cert_validation : record.fqdn]
}

resource "aws_lb_listener_certificate" "ccf" {
  count = var.enable_ccf ? 1 : 0

  listener_arn    = aws_lb_listener.https.arn
  certificate_arn = aws_acm_certificate_validation.ccf[0].certificate_arn
}

output "acm_certificate_arn" {
  description = "ARN of the ACM certificate attached to the ALB HTTPS listener."
  value       = aws_acm_certificate_validation.app.certificate_arn
}

output "ccf_acm_certificate_arn" {
  description = "ARN of the CCF ACM certificate attached to the HTTPS listener via SNI; empty when enable_ccf is false."
  value       = var.enable_ccf ? aws_acm_certificate_validation.ccf[0].certificate_arn : ""
}
