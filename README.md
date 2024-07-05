# Nori
Nori allows you to package, distribute and deploy your Terraform modules. Nori creates oci compatible images that can be tagged and pushed to any container registry. such as AWS ECR, Github Packages or Docker Hub. With nori you can easily deploy any terraform module with a single command.

![cli](assets/deploy-demo.gif)

## Requirements
- Go 1.21.5 or later
- Terraform or OpenTofu 1.5.0 or later

## Getting Started
To get started with Nori, You need to install the CLI by running the following command:
```bash
export PATH=$PATH:`go env GOPATH`/bin # Only needed if you havent set your GOPATH
go install github.com/eunanhardy/nori@latest
```

Setup your Nori configuration file by running the following command:
```bash
nori init
```
Setup your Nori config to use S3 as a backend:
```bash
nori init --backend s3://com.mycompany.terraform --backend-region eu-west-1
```

### Deploy
To Deploy your Terraform module, run the following command:
```bash
nori deploy 123456789012.dkr.ecr.eu-west-1.amazonaws.com/create-s3-bucket:v1 --values ./values.yaml
```

## Documentation
- [Building Modules For Deployability](docs/BUILDING_MODULES.md)
- [Options & Usage](docs/USAGE.md)
