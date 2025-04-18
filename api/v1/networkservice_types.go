package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:deepcopy-gen=true

// NetworkService defines a network service in NSM.
type NetworkService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkServiceSpec   `json:"spec,omitempty"`
	Status NetworkServiceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen=true

// NetworkServiceSpec defines the desired state of NetworkService.
type NetworkServiceSpec struct {
	ServiceType        string `json:"serviceType"`
	Endpoint           string `json:"endpoint"`
	Ports              []int  `json:"ports,omitempty"`
	LatencyRequirement int    `json:"latencyRequirement,omitempty"`
	Bandwidth          int    `json:"bandwidth,omitempty"`
}

// +k8s:deepcopy-gen=true

// NetworkServiceStatus defines the observed state of NetworkService.
type NetworkServiceStatus struct {
	State           string      `json:"state,omitempty"`
	ConnectionCount int         `json:"connectionCount,omitempty"`
	LastUpdated     metav1.Time `json:"lastUpdated,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:deepcopy-gen=true

// NetworkServiceList contains a list of NetworkService.
type NetworkServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkService `json:"items"`
}

// Initialize deepcopy functions for this type
func init() {
	SchemeBuilder.Register(&NetworkService{}, &NetworkServiceList{})
}
