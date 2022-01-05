/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package v1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IBMApplicationGatewaySpec defines the desired state of IBMApplicationGateway
type IBMApplicationGatewaySpec struct {
	Replicas      int32                                `json:"replicas"`
	Deployment    IBMApplicationGatewayDeployment      `json:"deployment"`
	Configuration []IBMApplicationGatewayConfiguration `json:"configuration"`
}

type IBMApplicationGatewayDeployment struct {
	ImageLocation      string                         `json:"image"`
	ImagePullPolicy    string                         `json:"imagePullPolicy"`
	ImagePullSecrets   []IBMApplicationGatewaySecrets `json:"imagePullSecrets"`
	ServiceAccountName string                         `json:"serviceAccountName"`
	Lang               string                         `json:"lang"`

        // +optional
	ConfigMapSuffix    string                         `json:"generatedConfigmapSuffix"`
	ReadinessProbe     IBMApplicationGatewayProbe     `json:"readinessProbe"`
	LivenessProbe      IBMApplicationGatewayProbe     `json:"livenessProbe"`
}

type IBMApplicationGatewaySecrets struct {
	Name string `json:"name"`
}

type IBMApplicationGatewayProbe struct {
        // +optional
	Command          string `json:"command"`

	InitDelay        int32  `json:"initialDelaySeconds"`
	Period           int32  `json:"periodSeconds"`
	FailureThreshold int32  `json:"failureThreshold"`
	SuccessThreshold int32  `json:"successThreshold"`
	TimeoutSeconds   int32  `json:"timeoutSeconds"`
}

type IBMApplicationGatewayConfiguration struct {
	Type              string                          `json:"type"`

        // +optional
	Name              string                          `json:"name"`

        // +optional
	DataKey           string                          `json:"dataKey"`

        // +optional
	Url               string                          `json:"url"`

        // +optional
	Headers           []IBMApplicationGatewayHeaders  `json:"headers"`

        // +optional
	Value             string                          `json:"value"`

        // +optional
	DiscoveryEndpoint string                          `json:"discoveryEndpoint"`

        // +optional
	Secret            string                          `json:"secret"`

        // +optional
	PostData          []IBMApplicationGatewayPostData `json:"postData"`
}

type IBMApplicationGatewayHeaders struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Name      string `json:"name"`
	SecretKey string `json:"secretKey"`
}

type IBMApplicationGatewayPostData struct {
        // +optional
	Value  string   `json:"value"`

	Name   string   `json:"name"`

        // +optional
	Values []string `json:"values"`
}

// IBMApplicationGatewayStatus defines the observed state of IBMApplicationGateway
type IBMApplicationGatewayStatus struct {
	Replicas int32      `json:"replicas"`
	PodNames []string   `json:"podNames"`
	PodSpec  v1.PodSpec `json:"pod_spec"`
	Version  string     `json:"version"`
	Status   bool       `json:"status"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// IBMApplicationGateway is the Schema for the ibmapplicationgateways API
type IBMApplicationGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IBMApplicationGatewaySpec   `json:"spec,omitempty"`
	Status IBMApplicationGatewayStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IBMApplicationGatewayList contains a list of IBMApplicationGateway
type IBMApplicationGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IBMApplicationGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IBMApplicationGateway{}, &IBMApplicationGatewayList{})
}
