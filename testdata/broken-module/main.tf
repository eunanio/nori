# This module has intentional errors for E2E testing

# Error 1: Reference to undefined variable
resource "random_pet" "pet" {
  count  = var.count_value
  prefix = var.undefined_variable  # This variable doesn't exist
}

# Error 2: Invalid attribute
resource "random_string" "str" {
  length           = 16
  invalid_attr     = true  # This attribute doesn't exist on random_string
}

# Error 3: Type mismatch - trying to use string where number expected
resource "random_password" "pass" {
  length = var.name  # name is a string, length expects number
}

