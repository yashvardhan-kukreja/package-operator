package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type PackageManifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PackageManifestSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase: Pending}
	Status PackageManifestStatus `json:"status,omitempty"`
}

// PackageManifestList contains a list of PackageManifests.
// +kubebuilder:object:root=true
type PackageManifestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PackageManifest `json:"items"`
}

// PackageManifestStatus defines the observed state of a PackageManifest.
type PackageManifestStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// When evaluating object state in code, use .Conditions instead.
	Phase PackageManifestStatusPhase `json:"phase,omitempty"`
}

type PackageManifestStatusPhase string

// Well-known PackageManifest Phases for printing a Status in kubectl,
// see deprecation notice in PackageManifestStatus for details.
const (
	// Default phase, when object is created and has been waiting for the child resource to be spun up successfully.
	PackageManifestStatusPhasePending PackageManifestStatusPhase = "Pending"
	// Available maps to Available condition == True, when the child resources have spun up successfully.
	PackageManifestStatusPhaseAvailable PackageManifestStatusPhase = "Available"
	// Failed phase, when tahe package manifest reconciler deterministically observes a failed attempt at provisioning the child resources.
	// For example, when it observes that a package-loader job was already spun up and it's now in a failed state
	PackageManifestStatusPhaseFailed PackageManifestStatusPhase = "Failed"
)

// PackageManifest specification.
type PackageManifestSpec struct {
	// the image digests corresponding to the image containing the contents of the package
	// this image will be unpacked by the package-loader to render the ObjectDeployment for propagating the installation of the package.
	// +kubebuilder:validation:Required
	PackageImage string `json:"packageImage"`

	// the namespace where the package intends to get installed
	// +kubebuilder:validation:Required
	TargetNamespace string `json:"targetNamespace"`
}

func init() {
	SchemeBuilder.Register(&PackageManifest{}, &PackageManifestList{})
}
