package controllers

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

// Creates an ECR client object from the AWS SDK
func CreateEcrClient() (*ecr.Client, error) {
	// load the default AWS config from ENV or shared files
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	client := ecr.NewFromConfig(cfg)
	return client, nil
}
