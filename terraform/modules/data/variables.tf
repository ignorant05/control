variable "project" {
  type = string
}

variable "environment" {
  type = string
}

variable "data_subnets" {
  type = list(string)
}

variable "security_group_id" {
  type = string
}

variable "db_instance_class" {
  type = string
}

variable "db_username" {
  type = string
}

variable "db_password" {
  type      = string
  sensitive = true
}
