package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:deepcopy-gen=true

// NetworkConnection defines a connection between endpoints in the mesh.
type NetworkConnection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkConnectionSpec   `json:"spec,omitempty"`
	Status NetworkConnectionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen=true

// NetworkConnectionSpec defines the desired state of NetworkConnection.
type NetworkConnectionSpec struct {
	Source         string `json:"source"`
	Destination    string `json:"destination"`
	ConnectionType string `json:"connectionType"`
	Priority       int    `json:"priority,omitempty"`
}

// +k8s:deepcopy-gen=true

// NetworkConnectionStatus defines the observed state of NetworkConnection.
type NetworkConnectionStatus struct {
	State       string            `json:"state,omitempty"`
	Established bool              `json:"established"`
	LastUpdated metav1.Time       `json:"lastUpdated,omitempty"`
	Metrics     ConnectionMetrics `json:"metrics,omitempty"`
}

// ConnectionMetrics contains performance metrics for the connection.
type ConnectionMetrics struct {
	Latency    int     `json:"latency,omitempty"`
	PacketLoss float64 `json:"packetLoss,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:deepcopy-gen=true

// NetworkConnectionList contains a list of NetworkConnection
type NetworkConnectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkConnection `json:"items"`
}

// Initialize deepcopy functions for this type
func init() {
	SchemeBuilder.Register(&NetworkConnection{}, &NetworkConnectionList{})
}
