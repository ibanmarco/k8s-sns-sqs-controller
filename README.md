# SNS to SQS Kubernetes controller
This is a _PoC_ of a very simple Kubernetes controller to publish a notification to an SNS topic with one or more subscription to SQS queues.

## Description
This controller creates an SNS topic that will be subscribed to a queue. It is possible to create multiple topics subscribed to multiple queues.
There are a few examples in `config/samples`.

This controller has been tested on non-EKS clusters and the values of the AWS secret/access keys have been taken from Vault, therefore a couple of variables need to be passed into the controller: `VAULT_HOST` and `VAULT_TOKEN`. However, if you are planning to use this controller on EKS you should consider IRSA.

Before applying the manifest, be sure that the CRD has been deployed:
```bash
❯ kubectl get crd | grep sqs
snstosqs.cninfra.ibanmarco.io                2023-11-01T13:44:12Z
```

Once the manifest has been applied and the resources created in AWS, you should be able to publish SNS notifications:
```bash
❯ kubectl get snstosqs subs-sample-02 | awsblur
NAME             STATE     AGE    AWS            REGION
subs-sample-02   Created   100s   xxxxxxxxxxxx   us-east-1

❯ aws sns publish --region us-east-1 --topic-arn arn:aws:sns:us-east-1:$AWS_ACCOUNT_ID:k8s-sns-01.fifo --message "This is only a test..." --message-group-id "group1" --message-deduplication-id "1"
{
    "MessageId": "eeba9cbb-7e66-5767-9438-c1071e9e6c05",
    "SequenceNumber": "10000000000000003000"
}

❯ aws sqs receive-message --queue-url https://sqs.us-east-1.amazonaws.com/$AWS_ACCOUNT_ID/k8s-sqs-01.fifo --region us-east-1 | awsblur
{
    "Messages": [
        {
            "MessageId": "6ae831b0-cecc-4c9c-8daa-91b57aafd3de",
            "ReceiptHandle": "AQEBR3zUrqVrnnxVFv9Mxnj48LO7PBoTivy7mUmc9Ud+lXmBesO5yDTuTuWQBhiff9nNc/K+WIqaehXY4ZODxf00NELPNBhcOzAJUCgxNredC1oOn0ZOLqvbtgDYIQ2Ut2pXsVQ05GTEvQML1Q0E/CWYGefDqtMPTEElm0pzMgVk/ANxjs5Yw5CXsd9Jz9otuHCPK8aYbuxTIkPwm8h7qxnebYiQkwpA6WX73KQRgs7w9dftCPb89WqoOJTYa21J6k7LhytY9Bso1YI3rSYtfL5Lrw==",
            "MD5OfBody": "6992fec32f984b63b470cec23ac6fef1",
            "Body": "{\n  \"Type\" : \"Notification\",\n  \"MessageId\" : \"eeba9cbb-7e66-5767-9438-c1071e9e6c05\",\n  \"SequenceNumber\" : \"10000000000000003000\",\n  \"TopicArn\" : \"arn:aws:sns:us-east-1:xxxxxxxxxxxx:k8s-sns-01.fifo\",\n  \"Message\" : \"This is only a test...\",\n  \"Timestamp\" : \"2023-11-01T14:20:40.869Z\",\n  \"UnsubscribeURL\" : \"https://sns.us-east-1.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=arn:aws:sns:us-east-1:xxxxxxxxxxxx:k8s-sns-01.fifo:881bffb8-bd9d-40e3-9b13-d5dd43a50e23\"\n}"
        }
    ]
}


```
