module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.24"

  cluster_name    = "${var.project}-${var.environment}"
  cluster_version = var.eks_cluster_version

  vpc_id     = var.vpc_id
  subnet_ids = var.private_subnets

  cluster_endpoint_public_access = true

  eks_managed_node_groups = {
    default = {
      instance_types = [var.node_instance_type]
      min_size       = 1
      max_size       = var.node_desired_size + 2
      desired_size   = var.node_desired_size
    }
  }

  enable_cluster_creator_admin_permissions = true
}
