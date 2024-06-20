resource "random_password" "db_password" {
  length           = 16
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

resource "aws_db_instance" "default" {
  allocated_storage    = var.storage_size
  db_name              = var.db_name
  engine               = "mysql"
  engine_version       = "8.0"
  instance_class       = "db.t3.micro"
  username             = "dbuser"
  password             = random_password.db_password.result
  parameter_group_name = "default.mysql8.0"
  skip_final_snapshot  = true
}
 
output "db_address" {
  value = aws_db_instance.default.endpoint
}

output "db_password" {
  value = random_password.db_password.result
}

output "db_username" {
  value = aws_db_instance.default.username
}