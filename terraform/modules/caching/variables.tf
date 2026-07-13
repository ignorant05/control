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

variable "cache_node_type" {
  type = string
}

variable "redis_auth_token" {
  type      = string
  sensitive = true
}
