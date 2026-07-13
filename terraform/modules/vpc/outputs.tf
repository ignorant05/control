output "vpc_id" {
  value = module.vpc.vpc_id
}

output "vpc_cidr" {
  value = module.vpc.vpc_cidr_block
}

output "public_subnets" {
  value = module.vpc.public_subnets
}

output "private_subnets" {
  value = module.vpc.private_subnets
}

output "data_subnets" {
  value = module.vpc.intra_subnets
}

output "private_route_table_ids" {
  value = module.vpc.private_route_table_ids
}
