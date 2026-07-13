variable "aws_region" {
  default = "us-east-1"
}

variable "project" {
  default = "control"
}

variable "environment" {
  default = "prod"
}

variable "vpc_cidr" {
  default = "10.0.0.0/16"
}

variable "azs" {
  default = ["us-east-1a", "us-east-1b"]
}

variable "eks_cluster_version" {
  default = "1.30"
}

variable "node_instance_type" {
  default = "t3.medium"
}

variable "node_desired_size" {
  default = 2
}

variable "db_instance_class" {
  default = "db.t4g.micro"
}

variable "cache_node_type" {
  default = "cache.t4g.micro"
}

variable "db_username" {
  default = "control"
}

variable "db_password" {
  sensitive = true
}

variable "github_repo" {
  description = "org/repo for the OIDC trust policy"
  default     = "ignorant05/control"
}

variable "public_subnet_cidrs" {
  type = list(string)
}

variable "private_subnet_cidrs" {
  type = list(string)
}

variable "data_subnet_cidrs" {
  type = list(string)
}

variable "redis_auth_token" {
  type = string
}
