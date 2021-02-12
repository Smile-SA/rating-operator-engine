package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RatingRuleModelSpec defines the desired state of RatingRuleModel
type RatingRuleModelSpec struct {
	Timeframe string `json:"timeframe"`
	Metric    string `json:"metric"`
	Name      string `json:"name"`
}

// RatingRuleModelStatus defines the observed state of RatingRuleModel
type RatingRuleModelStatus struct {
	Timeframe string `json:"timeframe"`
	Metric    string `json:"metric"`
	Name      string `json:"name"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RatingRuleModel is the Schema for the ratingrulemodels API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ratingrulemodels,scope=Namespaced
type RatingRuleModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RatingRuleModelSpec   `json:"spec,omitempty"`
	Status RatingRuleModelStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RatingRuleModelList contains a list of RatingRuleModel
type RatingRuleModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RatingRuleModel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RatingRuleModel{}, &RatingRuleModelList{})
}
