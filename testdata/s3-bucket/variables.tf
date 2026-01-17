variable "bucket_name" {
  description = "The name of the S3 bucket"
  type        = string
}

variable "versioning_enabled" {
  description = "Enable versioning on the bucket"
  type        = bool
  default     = false
}

variable "encryption_enabled" {
  description = "Enable server-side encryption"
  type        = bool
  default     = true
}

variable "block_public_access" {
  description = "Block all public access to the bucket"
  type        = bool
  default     = true
}

variable "force_destroy" {
  description = "Allow bucket to be destroyed even if it contains objects"
  type        = bool
  default     = false
}

variable "tags" {
  description = "Additional tags to apply to the bucket"
  type        = map(string)
  default     = {}
}

