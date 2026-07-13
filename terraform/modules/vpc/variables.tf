variable "aws_region" {
  type = string
}
variable "project" {
  type = string
}
variable "vpc_cidr" {
  type = string
}
variable "azs" {
  type = list(string)
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
