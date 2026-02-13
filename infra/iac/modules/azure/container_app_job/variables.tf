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
