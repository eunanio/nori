resource "aws_s3_bucket" "bucket" {
    bucket = var.bucket_name
    tags = {
        Name = var.bucket_name
    }
}

output "bucket_url" {
    value = "${aws_s3_bucket.bucket.bucket_domain_name}"
}