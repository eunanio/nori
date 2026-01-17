# Random password resource - generates a secure password stored only in state
resource "random_password" "main" {
  count   = var.resource_count
  length  = var.password_length
  special = var.password_special

  # Override special characters for compatibility
  override_special = "!@#$%&*()-_=+[]{}<>:?"
}

# Random string resource - generates a random alphanumeric string
resource "random_string" "main" {
  count   = var.resource_count
  length  = var.string_length
  special = false
  upper   = true
  lower   = true
  numeric = true
}

# Random UUID resource - generates a unique identifier
resource "random_uuid" "main" {
  count = var.resource_count
}

# Random pet resource - generates a human-readable name
resource "random_pet" "main" {
  count     = var.resource_count
  prefix    = var.pet_prefix
  separator = var.pet_separator
  length    = 2
}

# Terraform data resource - for lifecycle testing and storing arbitrary data
resource "terraform_data" "main" {
  count = var.resource_count

  input = {
    created_at  = timestamp()
    tags        = var.tags
    random_id   = random_uuid.main[count.index].result
    pet_name    = random_pet.main[count.index].id
    instance    = count.index
  }

  # Triggers replacement when input changes
  triggers_replace = [
    var.tags,
  ]
}

