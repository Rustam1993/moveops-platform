locals {
  # Container Apps Job secret names must be lowercase and use hyphens.
  secret_name_by_env = {
    for k, _ in var.secret_env : k => replace(lower(k), "_", "-")
  }
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

  registry {
    server   = var.registry_server
    identity = "system"
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
}
