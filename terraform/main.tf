terraform {
  required_version = ">= 1.9"

  backend "s3" {
    bucket       = "control-tfstate"
    key          = "eks/terraform.tfstate"
    region       = "us-east-1"
    use_lockfile = true
    encrypt      = true
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
  }
}

module "vpc" {
  source = "./modules/vpc"

  aws_region           = var.aws_region
  project              = var.project
  vpc_cidr             = var.vpc_cidr
  azs                  = var.azs
  public_subnet_cidrs  = var.public_subnet_cidrs
  private_subnet_cidrs = var.private_subnet_cidrs
  data_subnet_cidrs    = var.data_subnet_cidrs
}

module "compute" {
  source = "./modules/compute"

  project             = var.project
  environment         = var.environment
  eks_cluster_version = var.eks_cluster_version
  vpc_id              = module.vpc.vpc_id
  private_subnets     = module.vpc.private_subnets
  node_instance_type  = var.node_instance_type
  node_desired_size   = var.node_desired_size
}

module "security" {
  source = "./modules/security"

  project                    = var.project
  vpc_id                     = module.vpc.vpc_id
  eks_node_security_group_id = module.compute.node_security_group_id
}

module "data" {
  source = "./modules/data"

  project           = var.project
  environment       = var.environment
  data_subnets      = module.vpc.data_subnets
  security_group_id = module.security.rds_security_group_id
  db_instance_class = var.db_instance_class
  db_username       = var.db_username
  db_password       = var.db_password
}

module "caching" {
  source = "./modules/caching"

  project           = var.project
  environment       = var.environment
  data_subnets      = module.vpc.data_subnets
  security_group_id = module.security.redis_security_group_id
  cache_node_type   = var.cache_node_type
  redis_auth_token  = var.redis_auth_token
}

module "edge" {
  source = "./modules/edge"

  project         = var.project
  github_repo     = var.github_repo
  eks_cluster_arn = module.compute.cluster_arn
}
