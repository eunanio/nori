# S3 Bucket Module

A simple Terraform module for creating an AWS S3 bucket with common configurations.

## Features

- Server-side encryption (AES256)
- Versioning support
- Public access blocking
- Customizable tags

## Usage

```hcl
module "s3_bucket" {
  source = "oci://ghcr.io/myorg/s3-bucket:v1.0.0"

  bucket_name        = "my-application-bucket"
  versioning_enabled = true
  
  tags = {
    Environment = "production"
    Team        = "platform"
  }
}
```

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| bucket_name | The name of the S3 bucket | `string` | n/a | yes |
| versioning_enabled | Enable versioning on the bucket | `bool` | `false` | no |
| encryption_enabled | Enable server-side encryption | `bool` | `true` | no |
| block_public_access | Block all public access to the bucket | `bool` | `true` | no |
| force_destroy | Allow bucket to be destroyed even if it contains objects | `bool` | `false` | no |
| tags | Additional tags to apply to the bucket | `map(string)` | `{}` | no |

## Outputs

| Name | Description |
|------|-------------|
| bucket_id | The ID of the S3 bucket |
| bucket_arn | The ARN of the S3 bucket |
| bucket_domain_name | The bucket domain name |
| bucket_regional_domain_name | The bucket region-specific domain name |

