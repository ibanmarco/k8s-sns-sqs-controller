/*
Copyright 2023 IbanMarco.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snsTypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqsTypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"snstosqs/pkg/awsutils"
	"strings"
	"time"

	cninfrav1alpha1 "snstosqs/api/v1alpha1"
)

const (
	finalizer      = "snstosqs.cninfra.ibanmarco.io"
	policyDocument = `{
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Principal": {
							"AWS": "*"
						},
						"Action": "sqs:SendMessage",
						"Resource": "%s",
						"Condition": {
							"ArnEquals": {
								"aws:SourceArn": "%s"
							}
						}
					}
				]
			}`
)

var redrivePolicy struct {
	DeadLetterTargetArn string `json:"deadLetterTargetArn"`
	MaxReceiveCount     int    `json:"maxReceiveCount"`
}

// SnsToSqsReconciler reconciles a SnsToSqs object
type SnsToSqsReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	AwsCreds map[string]string
}

//+kubebuilder:rbac:groups=cninfra.ibanmarco.io,resources=snstosqs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cninfra.ibanmarco.io,resources=snstosqs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cninfra.ibanmarco.io,resources=snstosqs/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;create;update;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SnsToSqsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	instance := &cninfrav1alpha1.SnsToSqs{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		//log.Error(err, "unable to get resource")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// initial state
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if instance.Status.State == "" {
			instance.Status.State = cninfrav1alpha1.PendingState
			r.Status().Update(ctx, instance)
		}
		if instance.Status.AwsAccountId == "" {
			instance.Status.State = cninfrav1alpha1.PendingState
			r.Status().Update(ctx, instance)
		}
		controllerutil.AddFinalizer(instance, finalizer)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}

		// PendingState only during the creation process
		if instance.Status.State == cninfrav1alpha1.PendingState {
			log.Info("start creating resources")
			if err := r.createResources(ctx, instance); err != nil {
				instance.Status.State = cninfrav1alpha1.ErrorState
				r.Status().Update(ctx, instance)
				log.Error(err, "error creating resources")
				return ctrl.Result{}, err
			}
			instance.Status.State = cninfrav1alpha1.CreatedState
			r.Status().Update(ctx, instance)
		}

	} else {
		// when delete is called
		log.Info("start deleting resources")
		if err := r.deleteResources(ctx, instance); err != nil {
			instance.Status.State = cninfrav1alpha1.ErrorState
			r.Status().Update(ctx, instance)
			log.Error(err, "error deleting resources")
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(instance, finalizer)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SnsToSqsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cninfrav1alpha1.SnsToSqs{}).
		Complete(r)
}

// awsChecks does some checks before creating resources
func (r *SnsToSqsReconciler) awsChecks(ctx context.Context, snsToSqs *cninfrav1alpha1.SnsToSqs, kmsClient *kms.Client, sqsClient *sqs.Client, snsClient *sns.Client) error {
	log := log.FromContext(ctx)
	var awsKms, awsSqs, awsSns []string

	kmsResp, err := kmsClient.ListAliases(ctx, &kms.ListAliasesInput{})
	if err != nil {
		log.Error(err, "error listing KMS keys alias")
		return err
	}
	for _, k := range kmsResp.Aliases {
		awsKms = append(awsKms, strings.TrimPrefix(*k.AliasName, "alias/"))
	}

	sqsResp, err := sqsClient.ListQueues(ctx, &sqs.ListQueuesInput{})
	if err != nil {
		log.Error(err, "error listing SMS queues")
		return err
	}
	for _, k := range sqsResp.QueueUrls {
		awsSqs = append(awsSqs, k[strings.LastIndex(k, "/")+1:])
	}

	snsResp, err := snsClient.ListTopics(ctx, &sns.ListTopicsInput{})
	if err != nil {
		log.Error(err, "error listing SNS topics")
		return err
	}
	for _, k := range snsResp.Topics {
		topic := *k.TopicArn
		awsSns = append(awsSns, topic[strings.LastIndex(topic, ":")+1:])
	}

	for _, k := range snsToSqs.Spec.Sqs {
		if !slices.Contains(awsKms, k.KmsKeyId) {
			log.Error(err, "KMS key not found, needs to be created first")
			return err
		}
		if slices.Contains(awsSqs, k.QueueName) {
			log.Error(err, "SQS queue exists")
			return err
		}
	}

	for _, k := range snsToSqs.Spec.Sns {
		if !slices.Contains(awsKms, k.KmsKeyId) {
			log.Error(err, "kms key %v not found, needs to be created first")
			return err
		}
		if slices.Contains(awsSns, k.SnsName) {
			log.Error(err, "SNS topic exists")
			return err
		}
		//for _, e := range k.Endpoints {
		//	log.Info(e.SqsName)
		//	if !slices.Contains(awsSqs, e.SqsName) {
		//		log.Error(err, "SQS queue doesn't exist")
		//		return err
		//	}
		//}
	}
	return nil
}

// createResources is to manage the creation of the resources based on the CRD
func (r *SnsToSqsReconciler) createResources(ctx context.Context, snsToSqs *cninfrav1alpha1.SnsToSqs) error {
	log := log.FromContext(ctx)
	instance := &cninfrav1alpha1.SnsToSqs{}
	snsToSqs.Status.State = cninfrav1alpha1.CreatingState
	log.Info("starting to create resources")
	err := r.Status().Update(ctx, snsToSqs)
	if err != nil {
		return err
	}

	// getting all sessions
	log.Info("getting AWS clients")
	kmsClient, sqsClient, snsClient, stsClient, err := awsutils.GetAllClients(r.AwsCreds["accessKey"], r.AwsCreds["secretKey"], snsToSqs.Spec.AwsRegion)
	if err != nil {
		log.Error(err, "error getting AWS clients")
	}
	stsResp, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Error(err, "error getting caller identity")
	}
	snsToSqs.Status.AwsAccountId = *stsResp.Account

	// some checks before creating resources
	if err := r.awsChecks(ctx, snsToSqs, kmsClient, sqsClient, snsClient); err != nil {
		instance.Status.State = cninfrav1alpha1.ErrorState
		r.Status().Update(ctx, instance)
		log.Error(err, "error checking resources in manifest")
		return err
	}

	// AWS should normalize the definition of the tags for the services!
	dMapTag := map[string]string{
		"ManagedBy": "sns-to-sqs-k8s-controller",
	}
	dTypeTag := []snsTypes.Tag{
		{
			Key:   aws.String("ManagedBy"),
			Value: aws.String("sns-to-sqs-k8s-controller"),
		},
	}

	for _, q := range snsToSqs.Spec.Sqs {
		createQueueInput := &sqs.CreateQueueInput{
			QueueName: &q.QueueName,
			Attributes: map[string]string{
				"KmsMasterKeyId": fmt.Sprintf("alias/%v", q.KmsKeyId),
			},
			Tags: dMapTag,
		}

		// fifo queue requires fifo dlq if there is any
		if q.Fifo {
			createQueueInput.Attributes["FifoQueue"] = "true"
			//createQueueInput.Attributes["ContentBasedDeduplication"] = "true"
			//createQueueInput.Attributes["ContentBasedDeduplicationScope"] = "true"
			q.QueueName = fmt.Sprintf("%v.fifo", q.QueueName)
			q.DlqName = fmt.Sprintf("%v.fifo", q.DlqName)
		}

		dlqArn := fmt.Sprintf("arn:aws:sqs:%s:%s:%s", snsToSqs.Spec.AwsRegion, *stsResp.Account, q.DlqName)
		queues := []string{
			0: q.DlqName,
			1: q.QueueName}
		for i := range queues {
			log.Info("creating queue")
			createQueueInput.QueueName = aws.String(queues[i])
			if i == 1 {
				createQueueInput.Attributes["RedrivePolicy"] = fmt.Sprintf(`{"deadLetterTargetArn":"%s", "maxReceiveCount":10}`, dlqArn)
			}
			_, err := sqsClient.CreateQueue(ctx, createQueueInput)
			if err != nil {
				log.Error(err, "error creating queue")
				return err
			}
			time.Sleep(1 * time.Second)
		}
	}
	// create SNS
	time.Sleep(2 * time.Second)
	for _, n := range snsToSqs.Spec.Sns {
		// fifo queues require fifo topic for a correct subscription
		if n.Fifo {
			n.SnsName = fmt.Sprintf("%v.fifo", n.SnsName)
		}

		createTopicInput := &sns.CreateTopicInput{
			Name: aws.String(n.SnsName),
			Attributes: map[string]string{
				"KmsMasterKeyId": fmt.Sprintf("alias/%v", n.KmsKeyId),
			},
			Tags: dTypeTag,
		}

		if n.Fifo {
			createTopicInput.Attributes["FifoTopic"] = "true"
		}

		_, err := snsClient.CreateTopic(ctx, createTopicInput)
		if err != nil {
			log.Error(err, "error creating topic")
			return err
		}

		// setting up the endpoints
		for _, e := range n.Endpoints {
			listQueuesOutput, err := sqsClient.ListQueues(ctx, &sqs.ListQueuesInput{
				QueueNamePrefix: aws.String(e.SqsName),
			})
			if err != nil {
				log.Error(err, "error listing queues")
				return err
			}

			qName := func(qUrls []string, qName string) string {
				for _, qUrl := range listQueuesOutput.QueueUrls {
					urlSplit := strings.Split(qUrl, "/")
					if qName == urlSplit[len(urlSplit)-1] || fmt.Sprintf("%v.fifo", qName) == urlSplit[len(urlSplit)-1] {
						return urlSplit[len(urlSplit)-1]
					}
				}
				return "err"
			}(listQueuesOutput.QueueUrls, e.SqsName)
			if qName == "err" {
				log.Info("SNS endpoint not found!")
			}

			queueUrl := fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s", snsToSqs.Spec.AwsRegion, *stsResp.Account, qName)

			// found dlq associated to the queue if there is any
			dlqArn := func(ctx context.Context, q string, c *sqs.Client) string {
				a, err := c.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
					QueueUrl: aws.String(q),
					AttributeNames: []sqsTypes.QueueAttributeName{
						sqsTypes.QueueAttributeNameAll,
					},
				})
				if err != nil {
					log.Error(err, "queue not found")
					return ""
				}

				// Check if the RedrivePolicy attribute exists
				rpAttribute, found := a.Attributes["RedrivePolicy"]
				if found {
					err := json.Unmarshal([]byte(rpAttribute), &redrivePolicy)
					if err != nil {
						log.Error(err, "Error unmashaling the redrive policy")
						return ""
					}
					return redrivePolicy.DeadLetterTargetArn
				} else {
					return ""
				}

			}(ctx, queueUrl, sqsClient)

			queueArn := fmt.Sprintf("arn:aws:sqs:%s:%s:%s", snsToSqs.Spec.AwsRegion, *stsResp.Account, qName)
			subscribeInput := &sns.SubscribeInput{
				TopicArn: aws.String(fmt.Sprintf("arn:aws:sns:%s:%s:%s", snsToSqs.Spec.AwsRegion, *stsResp.Account, n.SnsName)),
				Protocol: aws.String("sqs"),
				Endpoint: aws.String(queueArn),
			}
			queueArns := []string{queueArn}

			if len(dlqArn) > 0 {
				subscribeInput.Attributes = map[string]string{
					"RedrivePolicy": fmt.Sprintf(`{"deadLetterTargetArn":"%s"}`, dlqArn),
				}
				queueArns = append(queueArns, dlqArn)
			}

			// create the subscription
			_, err = snsClient.Subscribe(ctx, subscribeInput)
			if err != nil {
				log.Error(err, "error creating subscription")
				return err
			}
			log.Info("Subscription created!")

			// add inline SQS policy allowing SNS to queue and dlq if exists
			for _, queueArn := range queueArns {
				queueUrl := fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s", snsToSqs.Spec.AwsRegion, *stsResp.Account, queueArn[strings.LastIndex(queueArn, ":")+1:])
				topicArn := fmt.Sprintf("arn:aws:sns:%s:%s:%s", snsToSqs.Spec.AwsRegion, *stsResp.Account, n.SnsName)
				policyDocument := fmt.Sprintf(policyDocument, queueArn, topicArn)

				setQueueAttributesInput := &sqs.SetQueueAttributesInput{
					QueueUrl: aws.String(queueUrl),
					Attributes: map[string]string{
						"Policy": policyDocument,
					},
				}

				_, err = sqsClient.SetQueueAttributes(context.TODO(), setQueueAttributesInput)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// deleteResources is to manage the deletion of the resources based on the CRD
func (r *SnsToSqsReconciler) deleteResources(ctx context.Context, snsToSqs *cninfrav1alpha1.SnsToSqs) error {
	log := log.FromContext(ctx)
	instance := &cninfrav1alpha1.SnsToSqs{}

	instance.Status.State = cninfrav1alpha1.DeletingState
	r.Status().Update(ctx, instance)

	// getting all sessions
	log.Info("getting AWS clients")
	_, sqsClient, snsClient, stsClient, err := awsutils.GetAllClients(r.AwsCreds["accessKey"], r.AwsCreds["secretKey"], snsToSqs.Spec.AwsRegion)

	stsResp, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Error(err, "error getting caller identity")
	}

	for _, n := range snsToSqs.Spec.Sns {
		if n.Fifo {
			n.SnsName = fmt.Sprintf("%v.fifo", n.SnsName)
		}
		topicArn := fmt.Sprintf("arn:aws:sns:%s:%s:%s", snsToSqs.Spec.AwsRegion, *stsResp.Account, n.SnsName)

		// delete SNS subscription
		for _, e := range n.Endpoints {
			if n.Fifo {
				e.SqsName = fmt.Sprintf("%v.fifo", e.SqsName)
			}
			// find the subscription and delete it
			allSubs, err := snsClient.ListSubscriptionsByTopic(ctx, &sns.ListSubscriptionsByTopicInput{
				TopicArn: aws.String(topicArn),
			})
			if err != nil {
				log.Error(err, "error getting all subscriptions")
			}

			for _, sub := range allSubs.Subscriptions {
				a := *sub.Endpoint
				subsEndpoint := a[strings.LastIndex(a, ":")+1:]
				if subsEndpoint == e.SqsName {
					_, err := snsClient.Unsubscribe(ctx, &sns.UnsubscribeInput{
						SubscriptionArn: sub.SubscriptionArn,
					})
					if err != nil {
						log.Error(err, "error deleting subscription")
					}
				}
			}
		}
		// delete SNS topic
		_, err = snsClient.DeleteTopic(ctx, &sns.DeleteTopicInput{
			TopicArn: aws.String(topicArn),
		})
		if err != nil {
			log.Error(err, "error deleting topic")
		}

	}

	// delete SQS queue
	for _, q := range snsToSqs.Spec.Sqs {
		if q.Fifo {
			q.QueueName = fmt.Sprintf("%v.fifo", q.QueueName)
			q.DlqName = fmt.Sprintf("%v.fifo", q.DlqName)
		}

		log.Info("removing queues")
		qUrls := []string{
			fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s", snsToSqs.Spec.AwsRegion, *stsResp.Account, q.QueueName),
			fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s", snsToSqs.Spec.AwsRegion, *stsResp.Account, q.DlqName),
		}

		for _, u := range qUrls {
			_, err := sqsClient.DeleteQueue(ctx, &sqs.DeleteQueueInput{
				QueueUrl: aws.String(u),
			})
			if err != nil {
				log.Error(err, "error deleting queue")
			}
		}
	}

	return nil
}
