output "pet_name" {
  value = random_pet.pet[0].id
}

output "random_string" {
  value = random_string.str.result
}

