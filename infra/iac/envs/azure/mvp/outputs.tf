output "web_url" {
  value = "https://${module.web.ingress_fqdn}"
}

output "api_internal_fqdn" {
  value = module.api.ingress_fqdn
}

output "postgres_fqdn" {
  value = module.postgres.fqdn
}

output "container_app_web_principal_id" {
  value = module.web.identity_principal_id
}

output "container_app_api_principal_id" {
  value = module.api.identity_principal_id
}

output "container_app_job_migrate_principal_id" {
  value = module.db_migrate.identity_principal_id
}

output "log_analytics_workspace_id" {
  value = module.logs.id
}
