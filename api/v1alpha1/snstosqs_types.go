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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PendingState  = "Pending"
	CreatedState  = "Created"
	CreatingState = "Creating"
	DeletingState = "Deleting"
	ErrorState    = "Error"
)

// SnsToSqsSpec defines the desired state of SnsToSqs
type SnsToSqsSpec struct {
	// AwsRegion is where the queues, topics and subscriptions will be created
	AwsRegion string `json:"awsRegion"`
	Sns       []Sns  `json:"sns"`
	Sqs       []Sqs  `json:"sqs"`
}

type Sns struct {
	// SnsName is the name of the topic
	SnsName string `json:"snsName"`
	// KmsKeyId is the alias of the KMS key and should be managed by IaC
	KmsKeyId string `json:"kmsKeyId"`
	// if the topic is Fifo, topic and queue need to be aligned
	Fifo bool `json:"fifo"`
	// Endpoints is a list of endpoints
	Endpoints []Endpoint `json:"endpoints"`
}

type Endpoint struct {
	// SqsName is the name of the queue that you want to receive notifications
	SqsName string `json:"sqsName"`
}

type Sqs struct {
	// QueueName is the name of the queue
	QueueName string `json:"queueName"`
	// if the queue is Fifo, topic and queue need to be aligned
	Fifo bool `json:"fifo"`
	// DlqName is the name of the dlq
	DlqName string `json:"dlqName"`
	// KmsKeyId is the alias of the KMS key and should be managed by IaC
	KmsKeyId string `json:"kmsKeyId"`
}

// SnsToSqsStatus defines the observed state of SnsToSqs
type SnsToSqsStatus struct {
	State        string `json:"state"`
	AwsAccountId string `json:"awsAccountId"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
//+kubebuilder:printcolumn:name="Aws",type=string,JSONPath=`.status.awsAccountId`
//+kubebuilder:printcolumn:name="Region",type=string,JSONPath=`.spec.awsRegion`

// SnsToSqs is the Schema for the snstosqs API
type SnsToSqs struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SnsToSqsSpec   `json:"spec,omitempty"`
	Status SnsToSqsStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SnsToSqsList contains a list of SnsToSqs
type SnsToSqsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SnsToSqs `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SnsToSqs{}, &SnsToSqsList{})
}
