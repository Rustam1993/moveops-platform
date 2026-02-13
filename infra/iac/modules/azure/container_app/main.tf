resource "azurerm_container_app" "this" {
  name                         = var.name
  resource_group_name          = var.resource_group_name
  container_app_environment_id = var.environment_id
  revision_mode                = "Single"

  identity {
    type = "SystemAssigned"
  }

  registry {
    server   = var.registry_server
    identity = "system"
  }

  ingress {
    external_enabled = var.ingress_external_enabled
    target_port      = var.container_port
    transport        = "auto"

    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }

  template {
    min_replicas = var.min_replicas
    max_replicas = var.max_replicas

    container {
      name   = var.container_name
      image  = var.image
      cpu    = var.cpu
      memory = var.memory

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
          secret_name = env.key
        }
      }

      liveness_probe {
        transport               = "HTTP"
        port                    = var.container_port
        path                    = var.liveness_path
        interval_seconds        = 10
        timeout                 = 2
        failure_count_threshold = 3
      }

      readiness_probe {
        transport               = "HTTP"
        port                    = var.container_port
        path                    = var.readiness_path
        interval_seconds        = 10
        timeout                 = 2
        failure_count_threshold = 3
      }
    }
  }

  dynamic "secret" {
    for_each = var.secret_env
    content {
      name  = secret.key
      value = secret.value
    }
  }
}
