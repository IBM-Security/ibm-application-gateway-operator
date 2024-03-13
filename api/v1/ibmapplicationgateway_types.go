/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LicenseAnnotation is the ILMT product code which will be added to managed pods.
type LicenseAnnotation string

const (
	Production LicenseAnnotation = "production"
)

// IBMApplicationGatewaySpec defines the desired state of IBMApplicationGateway
type IBMApplicationGatewaySpec struct {
	// Replicas is the number of desired replicas.
	// This is a pointer to distinguish between explicit zero and unspecified.
	// Defaults to 1.
	// +optional
	Replicas int32 `json:"replicas"`

	// Specification of the desired behavior of the Deployment.
	Deployment IBMApplicationGatewayDeployment `json:"deployment"`

	// The configuration information associated with the deployed container.
	Configuration []IBMApplicationGatewayConfiguration `json:"configuration"`
}

// Custom annotations to add to deployed IBM Appication Gateway container
type CustomAnnotation struct {
	// Key of the annotation to create.
	Key string `json:"key" protobuf:"bytes,64,rep,name=key"`
	// Value of the annotation to create.
	Value string `json:"value" protobuf:"bytes,64,rep,name=value"`
}

type IBMApplicationGatewayDeployment struct {

	// Docker image name.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	ImageLocation string `json:"image"`

	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +optional
	ImagePullPolicy string `json:"imagePullPolicy"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use. For example,
	// in the case of docker, only DockerConfig type secrets are honored.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets"`

	// ServiceAccountName is the name of the ServiceAccount to use to run this pod.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
	// +optional
	ServiceAccountName string `json:"serviceAccountName"`

	// The language in which log messages from the container will be generated.
	// +kubebuilder:default=C
	// +optional
	Lang string `json:"lang"`

	// A suffix which will be appended to the ConfigMap's which are created by
	// the operator.
	// +optional
	ConfigMapSuffix string `json:"generatedConfigmapSuffix"`

	// Periodic probe of container service readiness.
	// Container will be removed from service endpoints if the probe fails.
	// Cannot be updated.
	// +optional
	ReadinessProbe IBMApplicationGatewayProbe `json:"readinessProbe"`

	// Periodic probe of container liveness.
	// Container will be restarted if the probe fails.
	// Cannot be updated.
	// +optional
	LivenessProbe IBMApplicationGatewayProbe `json:"livenessProbe"`

	// +kubebuilder:validation:Enum=production
	// The IBM License Metric Tool annotation to add to the deployed containers.
	// This annotations is used by IBM to audit license usage for IBM licensed
	// products.
	// If no annotation is provided then the production ILMT annotation is used.
	// +optional
	LicenseAnnotation LicenseAnnotation `json:"licenseAnnotation,omitempty" protobuf:"bytes,14,opt,name=language,casttype=LicenseAnnotation"`

	// The set of custom annotations to add to the container being created.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=key
	// +patchStrategy=merge,
	CustomAnnotations []CustomAnnotation `json:"customAnnotations,omitempty" patchStrategy:"merge" patchMergeKey:"key" protobuf:"bytes,5,opt,name=customAnnotations"`
}

type IBMApplicationGatewayProbe struct {
	// Command is the command line to execute inside the container, the working directory for the
	// command  is root ('/') in the container's filesystem. The command is simply exec'd, it is
	// not run inside a shell, so traditional shell instructions ('|', etc) won't work. To use
	// a shell, you need to explicitly call out to that shell.
	// Exit status of 0 is treated as live/healthy and non-zero is unhealthy.
	// +optional
	Command string `json:"command"`

	// Number of seconds after the container has started before liveness probes are initiated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	InitDelay int32 `json:"initialDelaySeconds"`

	// How often (in seconds) to perform the probe.
	// Default to 10 seconds. Minimum value is 1.
	// +optional
	Period int32 `json:"periodSeconds"`

	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	// +optional
	FailureThreshold int32 `json:"failureThreshold"`

	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Defaults to 1. Must be 1 for liveness and startup. Minimum value is 1.
	// +optional
	SuccessThreshold int32 `json:"successThreshold"`

	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds"`
}

type IBMApplicationGatewayConfiguration struct {
	// The type of configuration data which is being provided.  Valid types
	// include: configmap, oidc_registration, web, literal.
	Type string `json:"type"`

	// The name of the configuration map to be used, when the type is configmap.
	// +optional
	Name string `json:"name"`

	// The name of the ConfigMap key which contains the configuration data.
	// Used when the type is configmap.
	// +optional
	DataKey string `json:"dataKey"`

	// The URL which is used to retrieve the configuration data.  Used when the
	// type is web.
	// +optional
	Url string `json:"url"`

	// Any headers which are associated with the request which is sent to
	// retrieve configuration data.  Used when type is web.
	// +optional
	Headers []IBMApplicationGatewayHeaders `json:"headers"`

	// The literal configuration data.  Used when type is literal.
	// +optional
	Value string `json:"value"`

	// The OIDC discovery endpoint.  Used when type is oidc_registration.
	// +optional
	DiscoveryEndpoint string `json:"discoveryEndpoint"`

	// The name of the secret which contains the credential information.  Used
	// when type is oidc_registration.
	// +optional
	Secret string `json:"secret"`

	// The POST data which is submitted as a part of the OIDC registration
	// flow.  Used when type is oidc_registration.
	// +optional
	PostData []IBMApplicationGatewayPostData `json:"postData"`
}

type IBMApplicationGatewayHeaders struct {
	// The type of data which is provided for the header.  Valid values are
	// either secret or literal.
	Type string `json:"type"`

	// The value of the header which is being added.  If a literal header type
	// is provided this field contains the actual value of the header.  If a
	// secret header type is provided this field contains the name of the
	// secret.
	Value string `json:"value"`

	// The name of the header which is being generated.
	Name string `json:"name"`

	// The name of the field within the secret which contains the value of
	// the header.
	// +optional
	SecretKey string `json:"secretKey"`
}

type IBMApplicationGatewayPostData struct {
	// The value of the post data.
	// +optional
	Value string `json:"value"`

	// The name of the post data.
	Name string `json:"name"`

	// An array of strings which will be used as the value of the post data.
	// +optional
	Values []string `json:"values"`
}

// IBMApplicationGatewayStatus defines the observed state of IBMApplicationGateway
type IBMApplicationGatewayStatus struct {
	// A boolean which is used to signify whether the resource has been
	// successfully created.
	// +kubebuilder:default=true
	Status bool `json:"status"`
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
