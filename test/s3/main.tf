resource "aws_s3_bucket" "bucket" {
    bucket = var.bucket_name
    tags = var.tags
}

output "bucket_url" {
    value = "${aws_s3_bucket.bucket.bucket_domain_name}"
}