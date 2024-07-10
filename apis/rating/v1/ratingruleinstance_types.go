package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RatingRuleInstanceSpec defines the desired state of RatingRuleInstance
type RatingRuleInstanceSpec struct {
	Timeframe string `json:"timeframe"`
	Metric    string `json:"metric"`
	Name      string `json:"name"`
	Cpu       string `json:"cpu,omitempty"`
	Memory    string `json:"memory,omitempty"`
	Price     string `json:"price,omitempty"`

}

// RatingRuleInstanceStatus defines the observed state of RatingRuleInstance
type RatingRuleInstanceStatus struct {
	Timeframe string `json:"timeframe"`
	Metric    string `json:"metric"`
	Name      string `json:"name"`
	Cpu       string `json:"cpu,omitempty"`
	Memory    string `json:"memory,omitempty"`
	Price     string `json:"price,omitempty"`

}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RatingRuleInstance is the Schema for the ratingruleinstances API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ratingruleinstances,scope=Namespaced
type RatingRuleInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RatingRuleInstanceSpec   `json:"spec,omitempty"`
	Status RatingRuleInstanceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RatingRuleInstanceList contains a list of RatingRuleInstance
type RatingRuleInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RatingRuleInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RatingRuleInstance{}, &RatingRuleInstanceList{})
}
