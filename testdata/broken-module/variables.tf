variable "name" {
  description = "A name for resources"
  type        = string
  # No default - this is required
}

variable "count_value" {
  description = "Number of resources"
  type        = number
  default     = 1
}

