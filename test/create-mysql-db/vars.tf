variable "db_name" {
    description = "The name of the database to create"
}

variable "storage_size" {
    description = "The size of the database storage in GB"
    default     = 50
}