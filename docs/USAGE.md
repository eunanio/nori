# Init
config to use S3 as a backend as a alternative to local backend:
```bash
nori init --backend s3://com.mycompany.terraform --backend-region eu-west-1
```

### Login
Example of login to a AWS ECR registry:
```bash
nori login --username AWS --password $(aws ecr get-login-password --region eu-west-1) 123456789012.dkr.ecr.eu-west-1.amazonaws.com
```
| Flag | Description |
| --- | --- |
| --username | The username to authenticate with the registry |
| --password | The password to authenticate with the registry |
| --password-stdin | Take the password from stdin |

### Plan
To create a preview of your module deployment, run the following command:
```bash
nori plan create-s3-bucket:v1 --values ./values.yaml
```
| Flag | Description |
| --- | --- |
| --values | The path to the values file |
| --release | The release id of the deployment to update |
| --provider | The path to the provider file |

### Apply
To Deploy your Terraform module, run the following command:
```bash
nori apply test-bucket create-s3-bucket:v1 --values ./values.yaml
```

| Flag | Description |
| --- | --- |
| --values | The path to the values file |
| --provider | The path to the provider file |

### Package
To package your Terraform module provide a valid tag and path to your module directory, tags that do not include a remote host will be considered local only e.g. `create-s3-bucket:v1` , run the following command:
```bash
nori package create-s3-bucket:v1 ./modules/s3-bucket
```

### Tag
Use tag to rename a module in the local registry:
```bash
nori tag create-s3-bucket:v1 123456789012.dkr.ecr.eu-west-1.amazonaws.com/create-s3-bucket:v2
```

### Push
To push your packaged module to a container registry, run the following command:
```bash
nori push 123456789012.dkr.ecr.eu-west-1.amazonaws.com/create-s3-bucket:v1
```
| Flag | Description |
| --- | --- |
| --insecure | Allow insecure connections to the registry |

### Pull
To pull your packaged module from a container registry, run the following command:
```bash
nori pull 123456789012.dkr.ecr.eu-west-1.amazonaws.com/create-s3-bucket:v1
```

| Flag | Description |
| --- | --- |
| --create | Exports the pulled image to the local working directory |

## List Packages
List all local packages
```bash
nori list
```

## Inspect Package
get details about a package from its manifest
```bash
nori inspect create-s3-bucket:v2
```

## Projects
Projects are the same as workspaces and make up the logical naming of groups of resoruces. when you run `nori init` the your project will be sent as `default`.
Setting project:
```bash
nori config --set-project storage
```
Get current Project
```bash
nori config project
```

## List Releases
List Releases that have been deployed. 
```bash
nori release list
```

## Destorying Resoruces
Destorys the state for all resources in a release.
```bash
nori destroy test-bucket
```