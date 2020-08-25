package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/api/core/v1"
)

// IBMApplicationGatewaySpec defines the desired state of IBMApplicationGateway
type IBMApplicationGatewaySpec struct {
	Replicas int32 `json:"replicas"`
	Deployment IBMApplicationGatewayDeployment `json:"deployment"`
	Configuration []IBMApplicationGatewayConfiguration `json:"configuration"`
}

type IBMApplicationGatewayDeployment struct {
	ImageLocation string `json:"image"`
	ImagePullPolicy string `json:"imagePullPolicy"`
	ImagePullSecrets []IBMApplicationGatewaySecrets `json:"imagePullSecrets"`
	ServiceAccountName string `json:"serviceAccountName"`
	Lang string `json:"lang"`
	ConfigMapSuffix string `json:"generatedConfigmapSuffix"`
	ReadinessProbe IBMApplicationGatewayProbe `json:"readinessProbe"`
	LivenessProbe IBMApplicationGatewayProbe `json:"livenessProbe"`
}

type IBMApplicationGatewaySecrets struct {
	Name string `json:"name"`
}

type IBMApplicationGatewayProbe struct {
	Command string `json:"command"`
	InitDelay int32 `json:"initialDelaySeconds"`
	Period int32 `json:"periodSeconds"`
	FailureThreshold int32 `json:"failureThreshold"`
	SuccessThreshold int32 `json:"successThreshold"`
	TimeoutSeconds int32 `json:"timeoutSeconds"`
}

type IBMApplicationGatewayConfiguration struct {
	Type string `json:"type"`
	Name string `json:"name"`
	DataKey string `json:"dataKey"`
	Url string `json:"url"`
	Headers []IBMApplicationGatewayHeaders `json:"headers"`
	Value string `json:"value"`
	DiscoveryEndpoint string `json:"discoveryEndpoint"`
	Secret string `json:"secret"`
	PostData []IBMApplicationGatewayPostData `json:"postData"`
}

type IBMApplicationGatewayHeaders struct {
	Type string `json:"type"`
	Value string `json:"value"`
	Name string `json:"name"`
	SecretKey string`json:"secretKey"`
}

type IBMApplicationGatewayPostData struct {
	Value string `json:"value"`
	Name string `json:"name"`
	Values []string `json:"values"`
}

// IBMApplicationGatewayStatus defines the observed state of IBMApplicationGateway
type IBMApplicationGatewayStatus struct {
	Replicas int32    `json:"replicas"`
	PodNames []string `json:"podNames"`
	PodSpec 		v1.PodSpec	`json:"pod_spec"`
	Version string    `json:"version"`
	Status bool       `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IBMApplicationGateway is the Schema for the ibmapplicationgateways API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ibmapplicationgateways,scope=Namespaced
type IBMApplicationGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IBMApplicationGatewaySpec   `json:"spec,omitempty"`
	Status IBMApplicationGatewayStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IBMApplicationGatewayList contains a list of IBMApplicationGateway
type IBMApplicationGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IBMApplicationGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IBMApplicationGateway{}, &IBMApplicationGatewayList{})
}
