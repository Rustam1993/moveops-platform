variable "name" {
  type = string
}

variable "location" {
  type = string
}

variable "resource_group_name" {
  type = string
}

variable "admin_user" {
  type      = string
  sensitive = true
}

variable "admin_password" {
  type      = string
  sensitive = true
}

variable "db_name" {
  type = string
}

variable "pg_version" {
  type    = string
  default = "16"
}

variable "sku_name" {
  type    = string
  default = "B_Standard_B1ms"
}

variable "storage_mb" {
  type    = number
  default = 32768
}

variable "backup_retention_days" {
  type    = number
  default = 7
}

variable "allow_azure_services" {
  type    = bool
  default = true
}

variable "allowed_ip_ranges" {
  type = list(object({
    name  = string
    start = string
    end   = string
  }))
  default = []
}
