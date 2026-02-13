output "id" {
  value = azurerm_postgresql_flexible_server.this.id
}

output "fqdn" {
  value = azurerm_postgresql_flexible_server.this.fqdn
}

output "db_name" {
  value = azurerm_postgresql_flexible_server_database.db.name
}
