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

// RepositorySpec defines the desired state of Repository
type RepositorySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// (Optional) The tag mutability setting for the repository.
	// +kubebuilder:default=IMMUTABLE
	// +kubebuilder:validation:Enum=MUTABLE;IMMUTABLE
	ImageTagMutability ImageTagMutability `json:"imageTagMutability"`

	// (Optional) The ImageScanningConfiguration for the repository.
	// +optional
	// +nullable
	ImageScanningConfiguration *ImageScanningConfiguration `json:"imageScanningConfiguration,omitempty"`

	// (Optional) The EncryptionConfiguration for the repository.
	// +optional
	// +nullable
	EncryptionConfiguration *EncryptionConfiguration `json:"encryptionConfiguration,omitempty"`
}

// The ImageTagMutability type defines MUTABLE or IMMUTABLE
type ImageTagMutability string

// The ImageScanningConfiguration for the repository.
type ImageScanningConfiguration struct {
	// Determines whether images are scanned after being pushed
	// +kubebuilder:default=true
	ScanOnPush bool `json:"scanOnPush"`
}

// The EncryptionType type defines AES256 or KMS
type EncryptionType string

type EncryptionConfiguration struct {
	// This member is required.
	// +kubebuilder:default=AES256
	// +kubebuilder:validation:Enum=AES256;KMS
	EncryptionType EncryptionType `json:"encryptionType"`

	// If you use the KMS encryption type, specify the CMK to use for encryption. The
	// alias, key ID, or full ARN of the CMK can be specified. The key must exist in
	// the same Region as the repository. If no key is specified, the default AWS
	// managed CMK for Amazon ECR will be used.
	KmsKey *string `json:"kmsKey,omitempty"`
}

// RepositoryStatus defines the observed state of Repository
type RepositoryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Full ARN of the repository
	RepositoryArn string `json:"registryArn"`

	// The registry ID where the repository was created
	RegistryId string `json:"registryId"`

	// The URI of the repository (in the form aws_account_id.dkr.ecr.region.amazonaws.com/repositoryName)
	RepositoryUri string `json:"repositoryUri"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Repository is the Schema for the repositories API
type Repository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepositorySpec   `json:"spec,omitempty"`
	Status RepositoryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RepositoryList contains a list of Repository
type RepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Repository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Repository{}, &RepositoryList{})
}
