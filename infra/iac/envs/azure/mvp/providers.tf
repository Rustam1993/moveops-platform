provider "azurerm" {
  features {}
  # GitHub OIDC principal doesn't have permissions to register arbitrary resource providers at subscription scope.
  # Providers are pre-registered for this MVP (see Phase 7 prereqs).
  resource_provider_registrations = "none"
}
