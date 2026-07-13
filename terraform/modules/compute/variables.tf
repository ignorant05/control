variable "project" {
  type = string
}

variable "environment" {
  type = string
}

variable "eks_cluster_version" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "private_subnets" {
  type = list(string)
}

variable "node_instance_type" {
  type = string
}

variable "node_desired_size" {
  type = number
}
