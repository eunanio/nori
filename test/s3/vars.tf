variable "bucket_name" {
  type        = string
  description = "description"
}

variable "tags" {
  type        = map(string)
  default = {
    Name        = "My bucket"
    Environment = "Dev"
  }
}