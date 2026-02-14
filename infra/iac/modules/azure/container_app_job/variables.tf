variable "name" {
  type = string
}

variable "location" {
  type = string
}

variable "resource_group_name" {
  type = string
}

variable "environment_id" {
  type = string
}

variable "image" {
  type = string
}

variable "registry_server" {
  type = string
}

variable "registry_username" {
  type    = string
  default = ""
}

variable "registry_password" {
  type      = string
  default   = ""
  sensitive = true
}

variable "cpu" {
  type    = number
  default = 0.25
}

variable "memory" {
  type    = string
  default = "0.5Gi"
}

variable "command" {
  type = list(string)
}

variable "env" {
  type    = map(string)
  default = {}
}

variable "secret_env" {
  type      = map(string)
  default   = {}
  sensitive = true
}
