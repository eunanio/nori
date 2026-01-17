output "passwords" {
  description = "Generated passwords (sensitive)"
  value       = random_password.main[*].result
  sensitive   = true
}

output "strings" {
  description = "Generated random strings"
  value       = random_string.main[*].result
}

output "uuids" {
  description = "Generated UUIDs"
  value       = random_uuid.main[*].result
}

output "pet_names" {
  description = "Generated pet names"
  value       = random_pet.main[*].id
}

output "resource_data" {
  description = "Data from terraform_data resources"
  value = [
    for td in terraform_data.main : td.output
  ]
}

output "resource_count" {
  description = "Number of resources created"
  value       = var.resource_count
}

output "summary" {
  description = "Summary of all generated resources"
  value = {
    total_resources = var.resource_count * 5 # 5 resource types
    pet_names       = random_pet.main[*].id
    uuids           = random_uuid.main[*].result
    strings         = random_string.main[*].result
  }
}

