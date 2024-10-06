# Building Modules For Deployability
Ideally your modules should be as batteries included as possible. This means that your module should be able to be deployed with minimal configuration. This may include setting provider defaults in your module. You should reduce the use of external dependencies in your module. This will make your module more portable and easier to deploy.

## Build your first module
To build your first module, you need to create a directory for your module. This directory should contain your Terraform configuration files. Here is an example of a simple module that creates an S3 bucket:
```hcl
# main.tf
provider "aws" {
    region = "eu-west-1"
}

resource "aws_s3_bucket" "bucket" {
    bucket = var.bucket_name
    tags = var.tags
}
```
Define your variables in a variables.tf file:
```hcl
# variables.tf
variable "bucket_name" {
    type = string
}

variable "tags" {
    type = map(string)
}
```
### Packaging your module
To package your module, run the `nori package` command with the path to your module directory and a tag for your module:
```bash
nori package create-s3-bucket:v1.0.0 ./modules/s3-bucket
```
This will create a deployable artifact that can be pushed to a container registry.

This is where the values file comes in. The values file is a yaml or json file that contains the values that will be passed to your Terraform module. These values map to the variables in your terraform module. Here is an example of a values file:
```yaml
bucket_name: com.mycompany.storage.bucket
tags: 
    Environment: dev
    Owner: Frontend
```

### Deploying your module
To deploy your module, run the `nori apply <release_id> <tag>` command with the path to your packaged module and the path to your values file:
```bash
nori apply test-bucket create-s3-bucket:v1.0.0 --values ./values.yaml
```
