locals {
  # Container Apps Job secret names must be lowercase and use hyphens.
  secret_name_by_env = {
    for k, _ in var.secret_env : k => replace(lower(k), "_", "-")
  }

  use_acr_admin_creds = var.registry_username != "" && var.registry_password != ""
  acr_password_secret = "acr-password"
}

resource "azurerm_container_app_job" "this" {
  name                         = var.name
  location                     = var.location
  resource_group_name          = var.resource_group_name
  container_app_environment_id = var.environment_id

  replica_timeout_in_seconds = 1800

  identity {
    type = "SystemAssigned"
  }

  dynamic "registry" {
    for_each = local.use_acr_admin_creds ? [1] : []
    content {
      server               = var.registry_server
      username             = var.registry_username
      password_secret_name = local.acr_password_secret
    }
  }

  dynamic "registry" {
    for_each = local.use_acr_admin_creds ? [] : [1]
    content {
      server   = var.registry_server
      identity = "system"
    }
  }

  manual_trigger_config {
    parallelism              = 1
    replica_completion_count = 1
  }

  template {
    container {
      name  = "job"
      image = var.image

      command = var.command
      cpu     = var.cpu
      memory  = var.memory

      dynamic "env" {
        for_each = var.env
        content {
          name  = env.key
          value = env.value
        }
      }

      dynamic "env" {
        for_each = var.secret_env
        content {
          name        = env.key
          secret_name = local.secret_name_by_env[env.key]
        }
      }
    }
  }

  dynamic "secret" {
    for_each = var.secret_env
    content {
      name  = local.secret_name_by_env[secret.key]
      value = secret.value
    }
  }

  dynamic "secret" {
    for_each = local.use_acr_admin_creds ? [1] : []
    content {
      name  = local.acr_password_secret
      value = var.registry_password
    }
  }
}
