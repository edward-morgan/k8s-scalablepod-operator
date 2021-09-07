/*
Copyright 2021.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Represents the status of a ScalablePod. SPStatus can either be:
// 1. Active - currently in use
// 2. Inactive - not bound to a Pod, requires spinning up
type SPStatus string

const (
	SPActive   SPStatus = "Active"
	SPInactive SPStatus = "Inactive"
)

// ScalablePodSpec defines the desired state of ScalablePod
type ScalablePodSpec struct {
	// +kubebuilder:validation:Minimum=0

	// Maximum time to wait between work before shutting down.
	MaxReadyTimeSec int32 `json:"maxReadyTimeSec"`

	PodImageName string `json:"podImageName"`

	PodImageTag string `json:"podImageTag"`
}

// ScalablePodStatus defines the observed state of ScalablePod
type ScalablePodStatus struct {
	// The current status of the ScalablePod
	Status *SPStatus `json:"status"`

	// When the workspace was last started
	StartedAt metav1.Time `json:"startedAt,omitempty"`

	// Reference to the pod this ScalablePod is bound to, if any
	// Can't use types.NamespacedName because it isn't json-annotated
	BoundPod *NamespacedName `json:"boundPod,omitempty"`

	// Whether or not this ScalablePod is requested to activate.
	Requested bool `json:"requested"`
}

type NamespacedName struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// ScalablePod is the Schema for the scalablepods API
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=sp
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Started At",type=string,JSONPath=`.status.startedAt`
type ScalablePod struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScalablePodSpec   `json:"spec,omitempty"`
	Status ScalablePodStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ScalablePodList contains a list of ScalablePod
type ScalablePodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScalablePod `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScalablePod{}, &ScalablePodList{})
}
