output "namespaces" {
  description = "Created namespaces"
  value       = [for ns in kubernetes_namespace.messenger : ns.metadata[0].name]
}

output "service_accounts" {
  description = "Created service accounts"
  value       = [for sa in kubernetes_service_account.messenger : sa.metadata[0].name]
}
