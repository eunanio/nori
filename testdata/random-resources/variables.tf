variable "password_length" {
  description = "Length of the generated password"
  type        = number
  default     = 16
}

variable "password_special" {
  description = "Include special characters in the password"
  type        = bool
  default     = true
}

variable "string_length" {
  description = "Length of the generated random string"
  type        = number
  default     = 8
}

variable "pet_prefix" {
  description = "Prefix for the random pet name"
  type        = string
  default     = "nori"
}

variable "pet_separator" {
  description = "Separator for the random pet name"
  type        = string
  default     = "-"
}

variable "resource_count" {
  description = "Number of random resources to create (for testing multiple resources)"
  type        = number
  default     = 1
}

variable "tags" {
  description = "Tags to apply (stored in terraform_data for testing)"
  type        = map(string)
  default     = {}
}

