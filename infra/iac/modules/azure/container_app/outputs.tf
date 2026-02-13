output "id" {
  value = azurerm_container_app.this.id
}

output "name" {
  value = azurerm_container_app.this.name
}

output "ingress_fqdn" {
  value = azurerm_container_app.this.ingress[0].fqdn
}

output "identity_principal_id" {
  value = azurerm_container_app.this.identity[0].principal_id
}
