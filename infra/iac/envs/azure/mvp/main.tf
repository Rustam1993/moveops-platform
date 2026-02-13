data "azurerm_resource_group" "rg" {
  name = var.resource_group_name
}

module "logs" {
  source              = "../../../modules/azure/log_analytics"
  name                = "${var.app_prefix}-logs"
  location            = var.location
  resource_group_name = data.azurerm_resource_group.rg.name
  retention_in_days   = var.log_retention_days
}

module "cae" {
  source                     = "../../../modules/azure/container_apps_env"
  name                       = "${var.app_prefix}-cae"
  location                   = var.location
  resource_group_name        = data.azurerm_resource_group.rg.name
  log_analytics_workspace_id  = module.logs.id
}

module "postgres" {
  source              = "../../../modules/azure/postgres_flexible"
  name                = "${var.app_prefix}-pg"
  location            = var.location
  resource_group_name = data.azurerm_resource_group.rg.name

  admin_user     = var.pg_admin_user
  admin_password = var.pg_admin_password
  db_name        = var.pg_db_name

  sku_name              = var.pg_sku_name
  storage_mb            = var.pg_storage_mb
  backup_retention_days = var.pg_backup_retention_days

  allow_azure_services = var.db_allow_azure_services
  allowed_ip_ranges    = var.db_allowed_ip_ranges
}

locals {
  # Container Apps ingress FQDNs are deterministic under the environment default domain.
  # Using these avoids a dependency cycle between web<->api config.
  web_fqdn = "${var.app_prefix}-web.${module.cae.default_domain}"
  api_fqdn = "${var.app_prefix}-api.${module.cae.default_domain}"

  # Internal base URL that the web app uses for server-side proxying.
  api_internal_base_url = "http://${local.api_fqdn}/api"

  # Public URL for callers.
  web_public_url = "https://${local.web_fqdn}"

  # TLS required for Flexible Server.
  database_url = "postgresql://${var.pg_admin_user}:${var.pg_admin_password}@${module.postgres.fqdn}:5432/${var.pg_db_name}?sslmode=require"
}

module "api" {
  source              = "../../../modules/azure/container_app"
  name                = "${var.app_prefix}-api"
  location            = var.location
  resource_group_name = data.azurerm_resource_group.rg.name
  environment_id      = module.cae.id

  image          = var.api_image
  container_port = 8080

  ingress_external_enabled = false

  cpu         = var.api_cpu
  memory      = var.api_memory
  min_replicas = 1
  max_replicas = 3

  registry_server = var.acr_server

  env = {
    APP_ENV            = "production"
    API_ADDR           = ":8080"
    SESSION_COOKIE_NAME = "mo_sess"
    SESSION_TTL_HOURS   = "12"
    COOKIE_SECURE       = "true"
    CSRF_ENFORCE        = "true"
    CORS_ALLOWED_ORIGINS = local.web_public_url

    # Do not auto-migrate in production; handled by Container Apps Job.
    MIGRATE_ON_START = "false"
    SEED_ON_START    = "false"
  }

  secret_env = {
    DATABASE_URL   = local.database_url
    SESSION_SECRET = var.session_secret
    CSRF_SECRET    = var.csrf_secret
  }

  liveness_path  = "/api/health"
  readiness_path = "/api/health"
}

module "web" {
  source              = "../../../modules/azure/container_app"
  name                = "${var.app_prefix}-web"
  location            = var.location
  resource_group_name = data.azurerm_resource_group.rg.name
  environment_id      = module.cae.id

  image          = var.web_image
  container_port = 3000

  ingress_external_enabled = true

  cpu         = var.web_cpu
  memory      = var.web_memory
  min_replicas = 0
  max_replicas = 2

  registry_server = var.acr_server

  env = {
    NODE_ENV            = "production"

    # Browser calls hit the web app, which proxies to API_INTERNAL_URL.
    NEXT_PUBLIC_API_URL = "/api"
    API_INTERNAL_URL    = local.api_internal_base_url

    PUBLIC_BASE_URL = local.web_public_url
  }

  liveness_path  = "/login"
  readiness_path = "/login"
}

module "db_migrate" {
  source              = "../../../modules/azure/container_app_job"
  name                = "${var.app_prefix}-db-migrate"
  location            = var.location
  resource_group_name = data.azurerm_resource_group.rg.name
  environment_id      = module.cae.id

  image           = var.api_image
  registry_server = var.acr_server

  command = ["/usr/local/bin/migrate", "-dir", "/app/migrations"]

  secret_env = {
    DATABASE_URL = local.database_url
  }
}
