/*
Copyright 2020 The argocd-clusterset authors.

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

// ClusterSetSpec defines the desired state of ClusterSet
type ClusterSetSpec struct {
	Selector ClusterSelector       `json:"selector,omitempty"`
	Template ClusterSecretTemplate `json:"template"`
}

type ClusterSelector struct {
	RoleARN string            `json:"roleARN,omitempty"`
	EKSTags map[string]string `json:"eksTags,omitempty"`
}

type ClusterSecretTemplate struct {
	Metadata ClusterSecretTemplateMetadata `json:"metadata"`
}

type ClusterSecretTemplateMetadata struct {
	Labels map[string]string `json:"labels"`
	Config ClusterSecretTemplateMetadataConfig `json:"config,omitempty"`
}

type ClusterSecretTemplateMetadataConfig struct {
	AWSAuthConfig ClusterSecretTemplateMetadataConfigAwsAuthConfig `json:"awsAuthConfig,omitempty"`
}

type ClusterSecretTemplateMetadataConfigAwsAuthConfig struct {
	RoleARN string `json:"roleARN,omitempty"`
}

// ClusterSetStatus defines the observed state of ClusterSet
type ClusterSetStatus struct {
	Clusters     ClusterSetStatusClusters `json:"clusters"`
	LastSyncTime metav1.Time              `json:"lastSyncTime"`
	Phase        string                   `json:"phase"`
	Reason       string                   `json:"reason"`
	Message      string                   `json:"message"`
}

// ClusterSetStatusClusters contains runner registration status
type ClusterSetStatusClusters struct {
	Names []string `json:"names,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.lastSyncTime",name=Last Sync,type=date

// ClusterSet is the Schema for the ClusterSet API
type ClusterSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSetSpec   `json:"spec,omitempty"`
	Status ClusterSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterSetList contains a list of ClusterSet
type ClusterSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterSet{}, &ClusterSetList{})
}
