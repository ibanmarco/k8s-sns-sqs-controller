package awsutils

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"time"
)

func createAWSSession(accessKey, secretKey, region string) (*aws.Config, error) {
	creds := aws.Credentials{
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		//CanExpire:       true,
		SessionToken: "", // fmt.Sprintf("snstosqs-controller-%s", strconv.Itoa(rs)),
		Expires:      time.Now().Add(10 * time.Minute),
	}

	credProvider := credentials.StaticCredentialsProvider{Value: creds}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credProvider),
	)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func kmsClient(accessKey, secretKey, region string) (*kms.Client, error) {
	cfg, err := createAWSSession(accessKey, secretKey, region)
	if err != nil {
		fmt.Println("error getting AWS credentials", err)
		return nil, err
	}
	return kms.NewFromConfig(*cfg), nil
}

func sqsClient(accessKey, secretKey, region string) (*sqs.Client, error) {
	cfg, err := createAWSSession(accessKey, secretKey, region)
	if err != nil {
		fmt.Println("error getting AWS credentials", err)
		return nil, err
	}
	return sqs.NewFromConfig(*cfg), nil
}

func snsClient(accessKey, secretKey, region string) (*sns.Client, error) {
	cfg, err := createAWSSession(accessKey, secretKey, region)
	if err != nil {
		fmt.Println("error getting AWS credentials", err)
		return nil, err
	}
	return sns.NewFromConfig(*cfg), nil
}

func stsClient(accessKey, secretKey, region string) (*sts.Client, error) {
	cfg, err := createAWSSession(accessKey, secretKey, region)
	if err != nil {
		fmt.Println("error getting AWS credentials", err)
		return nil, err
	}
	return sts.NewFromConfig(*cfg), nil
}

func GetAllClients(accessKey, secretKey, region string) (*kms.Client, *sqs.Client, *sns.Client, *sts.Client, error) {
	kmsClient, err := kmsClient(accessKey, secretKey, region)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	sqsClient, err := sqsClient(accessKey, secretKey, region)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	snsClient, err := snsClient(accessKey, secretKey, region)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	stsClient, err := stsClient(accessKey, secretKey, region)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return kmsClient, sqsClient, snsClient, stsClient, nil
}
