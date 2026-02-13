variable "name" {
  type = string
}

variable "location" {
  type = string
}

variable "resource_group_name" {
  type = string
}

variable "retention_in_days" {
  type    = number
  # Azure Log Analytics retention must be between 30 and 730 days.
  default = 30
}
