resource "random_string" "suffix" {
  length  = 8
  special = false
  upper   = false
  lower   = true
  numeric = true
}

module "s3" {
  source = "oci://ghcr.io/eunanio/oci-terraform-modules/s3?tag=v1.0.1"
  bucket_name        = "${var.project_id}.com.nori.test-${random_string.suffix.result}"
  tags = {
    Project = var.project_id
  }
}

