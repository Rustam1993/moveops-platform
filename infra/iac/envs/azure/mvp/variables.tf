variable "resource_group_name" {
  type    = string
  default = "rg-moveops-mvp"
}

variable "location" {
  type    = string
  default = "eastus"
}

variable "app_prefix" {
  type    = string
  default = "moveops"
}

variable "acr_server" {
  type    = string
  default = "moveacr.azurecr.io"
}

variable "api_image" {
  type        = string
  description = "Full image URI, e.g. moveacr.azurecr.io/moveops/api:<tag>"
}

variable "web_image" {
  type        = string
  description = "Full image URI, e.g. moveacr.azurecr.io/moveops/web:<tag>"
}

variable "pg_admin_user" {
  type      = string
  sensitive = true
}

variable "pg_admin_password" {
  type      = string
  sensitive = true
}

variable "pg_db_name" {
  type = string
}

variable "session_secret" {
  type      = string
  sensitive = true
}

variable "csrf_secret" {
  type      = string
  sensitive = true
}

# DB networking defaults (MVP)
variable "db_allow_azure_services" {
  type        = bool
  description = "When true, adds a 0.0.0.0 firewall rule (Azure services)."
  default     = true
}

variable "db_allowed_ip_ranges" {
  type = list(object({
    name  = string
    start = string
    end   = string
  }))
  description = "Optional additional firewall rules for Postgres (explicit ranges)."
  default     = []
}

# Budget defaults
variable "log_retention_days" {
  type    = number
  default = 7
}

variable "pg_sku_name" {
  type        = string
  description = "Postgres Flexible Server SKU"
  default     = "B_Standard_B1ms"
}

variable "pg_storage_mb" {
  type    = number
  default = 32768
}

variable "pg_backup_retention_days" {
  type    = number
  default = 7
}

variable "api_cpu" {
  type    = number
  default = 0.25
}

variable "api_memory" {
  type    = string
  default = "0.5Gi"
}

variable "web_cpu" {
  type    = number
  default = 0.25
}

variable "web_memory" {
  type    = string
  default = "0.5Gi"
}
