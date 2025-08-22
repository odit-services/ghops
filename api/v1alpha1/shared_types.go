package v1alpha1

type CrStatus struct {
	State             string `json:"state,omitempty" yaml:"state,omitempty"`
	LastAction        string `json:"lastAction,omitempty" yaml:"lastAction,omitempty"`
	LastMessage       string `json:"lastMessage,omitempty" yaml:"lastMessage,omitempty"`
	LastReconcileTime string `json:"lastReconcileTime,omitempty" yaml:"lastReconcileTime,omitempty"`
	CurrentRetries    int    `json:"currentRetries,omitempty" yaml:"currentRetries,omitempty"`
}
