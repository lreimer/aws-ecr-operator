/*
MIT License

Copyright (c) 2021 M.-Leander Reimer

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RepositoryLifecycleSpec defines the desired state of RepositoryLifecycle
type RepositoryLifecycleSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The name of the repository to receive the policy.
	RepositoryName string `json:"repositoryName"`

	// The LifecyclePolicyText JSON text. This member is required.
	LifecyclePolicyText string `json:"lifecyclePolicyText"`
}

// RepositoryLifecycleStatus defines the observed state of RepositoryLifecycle
type RepositoryLifecycleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RepositoryLifecycle is the Schema for the repositorylifecycles API
type RepositoryLifecycle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepositoryLifecycleSpec   `json:"spec,omitempty"`
	Status RepositoryLifecycleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RepositoryLifecycleList contains a list of RepositoryLifecycle
type RepositoryLifecycleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RepositoryLifecycle `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RepositoryLifecycle{}, &RepositoryLifecycleList{})
}
