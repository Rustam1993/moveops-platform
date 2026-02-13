resource "azurerm_container_app_environment" "this" {
  name                = var.name
  location            = var.location
  resource_group_name = var.resource_group_name

  # azurerm ~>4.x no longer accepts a workspace key here.
  log_analytics_workspace_id = var.log_analytics_workspace_id
}
