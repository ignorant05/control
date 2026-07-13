resource "aws_db_subnet_group" "this" {
  name       = "${var.project}-db"
  subnet_ids = var.data_subnets
}

resource "aws_db_instance" "postgres" {
  identifier              = "${var.project}-${var.environment}"
  engine                  = "postgres"
  engine_version          = "16"
  instance_class          = var.db_instance_class
  allocated_storage       = 20
  storage_type            = "gp3"
  db_name                 = "control"
  username                = var.db_username
  password                = var.db_password
  db_subnet_group_name    = aws_db_subnet_group.this.name
  vpc_security_group_ids  = [var.security_group_id]
  publicly_accessible     = false
  skip_final_snapshot     = false
  backup_retention_period = 7
  deletion_protection     = true
}
