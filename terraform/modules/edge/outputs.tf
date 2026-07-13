output "github_actions_role_arn" {
  value = aws_iam_role.github_actions.arn
}

output "waf_web_acl_arn" {
  value = aws_wafv2_web_acl.api.arn
}
