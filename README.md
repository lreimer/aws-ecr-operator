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
    app: demo-microservice
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
# see https://book.kubebuilder.io/reference/markers/crd-validation.html
$ make generate && make manifests
$ make build

# run operator locally outside the cluster
# see https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
# see https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html
# THESE ARE DUMMY CREDENTIALS :-) !
$ export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
$ export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
$ export AWS_DEFAULT_REGION=eu-central-1
$ make install run

# try to create an ECR and do cleanup afterwards
$ kubectl apply -k config/samples
$ kubectl delete -k config/samples

# for (local) in-cluster deployment
# you need to add the above environment variables to a hidden .env.secret file
# MAKE SURE NOT TO COMMIT THIS FILE :-) !
$ echo AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE >> config/manager/.env.secret
$ echo AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY >> config/manager/.env.secret
$ echo AWS_DEFAULT_REGION=eu-central-1 >> config/manager/.env.secret

# build Docker image locally (optional) and deploy
$ make docker-build
$ make deploy

# try to create an ECR and do cleanup afterwards
$ kubectl apply -k config/samples
$ kubectl delete -k config/samples
```

## Maintainer

M.-Leander Reimer (@lreimer), <mario-leander.reimer@qaware.de>

## License

This software is provided under the MIT open source license, read the `LICENSE`
file for details.

