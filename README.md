# AWS ECR Operator

A K8s operator to manage an AWS ECR Repository as a custom resource.

```yaml
apiVersion: ecr.aws.cloud.qaware.de/v1beta1
kind: Repository
metadata:
  # name of the ECR repository
  name: demo-microservice
  # will be used as repository tags
  labels:
    app: nginx
spec:
  # valid values are MUTABLE or IMMUTABLE. Defaults to IMMUTABLE
  imageTagMutability: IMMUTABLE
  imageScanningConfiguration:
    scanOnPush: true
  encryptionConfiguration:
    # valid values are AES256 and KMS. Defaults to AES256
    encryptionType: AES256
    # the ARN of the KMS key to use
    # kmsKey: 
```

## Development

```bash
# perform skaffolding with the Operator SDK
$ operator-sdk init --project-version=3 --domain aws.cloud.qaware.de --repo github.com/lreimer/aws-ecr-operator
$ operator-sdk create api --group ecr --version=v1beta1 --kind Repository --resource --controller

# install AWS SDK for Go v2
$ go get github.com/aws/aws-sdk-go-v2
$ go get github.com/aws/aws-sdk-go-v2/config
$ go get github.com/aws/aws-sdk-go-v2/service/ecr

# define CRD in api/repository_types.go
$ make generate && make manifests
$ make build
```

## Maintainer

M.-Leander Reimer (@lreimer), <mario-leander.reimer@qaware.de>

## License

This software is provided under the MIT open source license, read the `LICENSE`
file for details.

