variable "name" {
  type = string
}

variable "resource_group_name" {
  type = string
}

variable "location" {
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

variable "container_port" {
  type = number
}

variable "container_name" {
  type    = string
  default = "app"
}

variable "ingress_external_enabled" {
  type = bool
}

variable "cpu" {
  type = number
}

variable "memory" {
  type = string
}

variable "min_replicas" {
  type    = number
  default = 0
}

variable "max_replicas" {
  type    = number
  default = 1
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

variable "liveness_path" {
  type    = string
  default = "/"
}

variable "readiness_path" {
  type    = string
  default = "/"
}
