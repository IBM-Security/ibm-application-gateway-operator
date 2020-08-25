package ibmapplicationgateway

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"context"
	"strconv"
	"sort"

	"github.com/ghodss/yaml"
	v1beta1 "k8s.io/api/admission/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/api/errors"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()

	// (https://github.com/kubernetes/kubernetes/issues/57982)
	defaulter = runtime.ObjectDefaulter(runtimeScheme)
)

var ignoredNamespaces = []string{
	metav1.NamespaceSystem,
	metav1.NamespacePublic,
}

const (
	admissionWebhookAnnotationInjectKey = "ibm-application-gateway.security.ibm.com/"
	imageAnnot = "ibm-application-gateway.security.ibm.com/deployment.image"
	servPort = "ibm-application-gateway.security.ibm.com/service.port"
	confPrefix = "ibm-application-gateway.security.ibm.com/configuration."
	envPrefix = "ibm-application-gateway.security.ibm.com/env."
	servAnnot = "ibm-application-gateway.security.ibm.com/serviceName"
	cmAnnot = "ibm-application-gateway.security.ibm.com/configMapName"
	volumeName = "ibm-application-gateway-config"
)

var updateRequiredAnnotations = []string{
	envPrefix,
	confPrefix,
	servPort,
	imageAnnot,
}

type WebhookServer struct {
	server        *http.Server
	client        client.Client
	scheme        *runtime.Scheme
}

type IAGConfigElement struct {
	Name string
	Type string
	DataKey string
	Value string
	Url string 
	Headers []IAGHeader
	Order  int
}

// Webhook Server parameters
type WhSvrParameters struct {
	port           int    // webhook server port
	certFile       string // path to the x509 certificate for https
	keyFile        string // path to the x509 private key matching `CertFile`
}

type Config struct {
	Containers []corev1.Container `yaml:"containers"`
	Volumes    []corev1.Volume    `yaml:"volumes"`
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1beta1.AddToScheme(runtimeScheme)
	// defaulting with webhooks:
	// https://github.com/kubernetes/kubernetes/issues/57982
	_ = v1.AddToScheme(runtimeScheme)
}

// (https://github.com/kubernetes/kubernetes/issues/57982)
func applyDefaultsWorkaround(containers []corev1.Container, volumes []corev1.Volume) {
	defaulter.Default(&corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: containers,
			Volumes:    volumes,
		},
	})
}

func loadConfig(configFile string) (*Config, error) {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("New configuration: sha256sum %x", sha256.Sum256(data)))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

/*
 * Function checks whether the target resoured need to be mutated
 */
func mutationRequired(whsvr *WebhookServer, ignoredList []string, metadata *metav1.ObjectMeta, 
	                  isUpdate bool, ns string, appName string, isPod bool) (bool, []string) {

	log.Info("WebhookServer: mutationRequired")

	var annotationChanges []string

	// skip special kubernete system namespaces
	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			log.Info(fmt.Sprintf("Skip mutation for %v for it's in special namespace:%v", metadata.Name, metadata.Namespace))
			return false, nil
		}
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	// If the target resource has any iag annotations then mutate it.
	var required bool = false
	for key, _ := range annotations {
		if strings.HasPrefix(key, admissionWebhookAnnotationInjectKey) {
			required = true
		}
	}

	// For update need to make sure that something meaningful has changed
	if isUpdate == true {

		var err error
		var annots map[string]string

		if isPod {
			obj := &corev1.Pod{}
			err = whsvr.client.Get(context.TODO(), types.NamespacedName{Name: appName, Namespace: ns}, obj)
			annots = obj.Annotations
		} else {
			obj := &appsv1.Deployment{}
			err = whsvr.client.Get(context.TODO(), types.NamespacedName{Name: appName, Namespace: ns}, obj)
			annots = obj.Annotations
		}
		
		if err != nil {
			log.Error(err, "Could not find object to update : " + appName)
			return false, nil
		}

		for key, value := range annots {

			if strings.HasPrefix(key, admissionWebhookAnnotationInjectKey) {
				if value != annotations[key] {
					
					// Pods do not support env updates
					if isPod && strings.HasPrefix(key, envPrefix) {
						continue
					}
					
					annotationChanges = append(annotationChanges, key)

					// Check to see if mutate is required yet
					if required != true {
						for _, impAnnot := range updateRequiredAnnotations {
							if strings.HasPrefix(key, impAnnot) {
								required = true
							}
						}
					}
				}
			}
		}

		// Any not handled
		for key, _ := range annotations {
			if strings.HasPrefix(key, admissionWebhookAnnotationInjectKey) {
				// Pods do not support env updates
				if isPod && strings.HasPrefix(key, envPrefix) {
					continue
				}

				// If the new annotations does not contain old value, its been deleted 
				if annots[key] == "" {
					annotationChanges = append(annotationChanges, key)
				
					// Check to see if mutate is required yet
					if required != true {
						for _, impAnnot := range updateRequiredAnnotations {
							if strings.HasPrefix(key, impAnnot) {
								required = true
							}
						}
					}
				}
			}
		}
	}

	log.Info(fmt.Sprintf("Mutation policy for %v/%v: required:%v", metadata.Namespace, metadata.Name, required))
	return required, annotationChanges
}

/*
 * Do some validation of the annotation entries to try and catch errors before changes are made.
 */
func validateAnnotations(annots map[string]string) (error, []IAGConfigElement) {

	log.Info("WebhookServer: validateAnnotations")

	// Image is required
	if annots[imageAnnot] == "" {
		return fmt.Errorf("No IBM Application Gateway image has been specified."), nil
	}
	
	configElements, err := getConfigElements(annots)
	if err != nil {
		return err, nil
	}

	return nil, configElements
}

/*
 * Parse the annotations list and return a list of the config source entries only.
 */
func getConfigElements(annots map[string]string) ([]IAGConfigElement, error) {

	log.Info("WebhookServer: getConfigElements")

	var err error

	var configElements []IAGConfigElement
	cfgAnnotations := make(map[string]string)
	configNames := make(map[string]struct{})
	hdrNames := make(map[string]struct{})

	// First get all of the config annotations
	for key, value := range annots {

		// Configuration keys will look like ibm-application-gateway.security.ibm.com/configuration.<name>.<vals>
		if strings.HasPrefix(key, confPrefix) {
			realKey := strings.TrimPrefix(key, confPrefix)

			cfgAnnotations[realKey] = value

			// Split out the name and vals. Need to save each unique name
			configParts := strings.Split(realKey, ".")

			configNames[configParts[0]] = struct{}{}

			// Also save each unique header name
			headerKey := strings.TrimPrefix(realKey, configParts[0])

			if strings.HasPrefix(headerKey, ".header.") {
				headerKey = strings.TrimPrefix(headerKey, ".header.")

				// Next part is the header name
				headerParts := strings.Split(headerKey, ".")

				hdrNames[headerParts[0]] = struct{}{}
			}
		}
	}

	// Now for each unique name get the required config
	for name := range(configNames) {
		var currElem IAGConfigElement

		currElem.Type = cfgAnnotations[name + ".type"]
		currElem.Order, err = strconv.Atoi(cfgAnnotations[name + ".order"])
		if err != nil {
		    return nil, fmt.Errorf("Configuration entry has an invalid order value : " + cfgAnnotations[name + ".order"])
		}

		switch currElem.Type {
			case "configmap":
				// Config map requires name, datakey and order
				currElem.Name = cfgAnnotations[name + ".name"]
				currElem.DataKey = cfgAnnotations[name + ".dataKey"]
				
			case "web":
				// Web has a url , order and headers
				currElem.Url = cfgAnnotations[name + ".url"]

				var headers []IAGHeader

				// There can be multiple headers defined in the form
				// configuration.sample.header.<name>.<vals>
				for hdrName := range(hdrNames) {

					var hdrPrefix = name + ".header." + hdrName

					var currHdr IAGHeader

					currHdr.Type = cfgAnnotations[hdrPrefix + ".type"]

					if currHdr.Type != "" && currHdr.Type != "secret" && currHdr.Type != "literal" {
						return nil, fmt.Errorf("Configuration entry has an invalid header type : " + currHdr.Type)
					}

					if currHdr.Type != "" {
						// Valid for this entry
						currHdr.Name = cfgAnnotations[hdrPrefix + ".name"]
						currHdr.Value = cfgAnnotations[hdrPrefix + ".value"]
						currHdr.SecretKey = cfgAnnotations[hdrPrefix + ".secretKey"]

						headers = append(headers, currHdr)
					}
				}

				currElem.Headers = headers

			default:
				return nil, fmt.Errorf("Configuration entry has an invalid type : " + currElem.Type)
		}

		configElements = append(configElements, currElem)
	}

	// Make sure there is at least one
	if len(configElements) < 1 {
		return nil, fmt.Errorf("No configuration entries specified in the annotations.")
	}

	return configElements, nil
}

/*
 * Function sorts the config source array and creates the merged master IAG configmap.
 */
func createIAGConfig(whsvr *WebhookServer, req *v1beta1.AdmissionRequest, configElements []IAGConfigElement, update bool, cmName string) (string, error) {

	log.Info("WebhookServer: createIAGConfig")

	// Sort via the order fields
	sort.SliceStable(configElements, func(first, second int) bool {
	    return configElements[first].Order < configElements[second].Order
	})

	// Merge all of the entries
	return mergeIAGConfig(whsvr, configElements, req.Namespace, req, update, cmName)
}

/*
 * Function creates the merged master IAG configmap.
 */
func mergeIAGConfig(whsvr *WebhookServer, configElements []IAGConfigElement, ns string, req *v1beta1.AdmissionRequest, update bool, cmName string) (string, error) {

	log.Info("WebhookServer : mergeIAGConfig")

	master := make(map[string]interface {})
	var err error

	//var nsn types.NamespacedName{Name: "dummy", NameSpace: ns}

	for _, element := range configElements {
		switch element.Type {
			case "configmap":
				// Handle configmap entry
				master, err = handleIAGConfigMap(whsvr, element.Name, element.DataKey, ns, master)
				if err != nil {
					log.Info("Error encountered attempting to merge a config map : " + element.Name)
					return "", err
				}
			case "web":
				// Handle web entry
				master, err = handleWebEntryMerge(whsvr.client, types.NamespacedName{Name: "dummy", Namespace: ns}, 
					                              element.Url, element.Headers, master)
				if err != nil {
					log.Info("Error encountered attempting to merge a web config : " + element.Url)
					return "", err
				}
		}
	}

	// Marshal the object to a yaml byte array
	masterYaml, err := yaml.Marshal(validateStringKeysFromString(master))
	if err != nil {
		log.Error(err, "failed to marshal the YAML master configuration")
		return "", err
	}

	var retName string

	// First create the new configmap	
	configMap := getNewConfigMap(getWebhookConfigMapName(req), getAppName(req), ns, string(masterYaml))
	err = whsvr.client.Create(context.TODO(), configMap)
	if err != nil {
		log.Error(err, "Error encountered while attempting to create the IBM Application Gateway config map")
		return "", err
	}

	retName = configMap.Name

	// Then delete the old one
	deleteConfigMap(whsvr, req, cmName)

	return retName, nil
}

/*
 * Function retrieves a config map source and merges the data with the current master source.
 */
func handleIAGConfigMap(whsvr *WebhookServer, configMap string, dataKey string, ns string, masterConfig map[string]interface {}) (map[string]interface {}, error) {
	
	log.Info("WebhookServer : handleIAGConfigMap")

	// Fetch the config map
	configMapFound := &corev1.ConfigMap{}
	err := whsvr.client.Get(context.TODO(), types.NamespacedName{Name: configMap, Namespace: ns}, configMapFound)
	if err != nil {
		log.Error(err, "Could not find config map : " + configMap)
		return nil, err
	}

	// Get the config map data pointed at by the data key
	cmData := configMapFound.Data[dataKey]
	
	masterConfig, err = handleYamlDataMerge(cmData, masterConfig)
	if err != nil {
		log.Error(err, "Failed to merge the configmap data")
		return nil, err
	}

	return masterConfig, nil
}

/*
 * Function creates a new service to expose the IAG 8443 port
 */
func addIAGService(whsvr *WebhookServer, annots map[string]string, req *v1beta1.AdmissionRequest) (string, error) {

	log.Info("WebhookServer : addIAGService")

	service := newService(annots, req)

	err := whsvr.client.Create(context.TODO(), service)
	if err != nil {
		log.Error(err, "Error encountered while creating the service")
		return "", err
	}

	return  service.Name, nil
}

/*
 * Function updates the service to expose the IAG 8443 port. If no service port is passed in the
 * annotations this will delete the old service.
 */
func updateIAGService(whsvr *WebhookServer, annots map[string]string, req *v1beta1.AdmissionRequest) (string, error) {

	log.Info("WebhookServer : updateIAGService")

	var sName string
	var err error

	// Only create the new service if the port has been specified
	if annots[servPort] != "" {
		// First create the new service
		sName, err = addIAGService(whsvr, annots, req)
		if err != nil {
			return "", err
		}
	}

	// Delete the old service
	err = deleteService(whsvr, req, annots[servAnnot])
	if err != nil {
		log.Error(err, "An error was encountered while attempting to delete the old service.")
	}

	return  sName, nil
}

/*
 * Function retrieves the base service name.
 */
func getServiceName(req *v1beta1.AdmissionRequest) string {

	name := req.Name
	if req.Kind.Kind == "Pod" {
		name = name + "-pod"
	}

	return strings.ToLower(name + "-ibm-application-gateway-sidecar-svc")
}

/*
 * Function retrieves the base application name.
 */
func getAppName(req *v1beta1.AdmissionRequest) (string) {

	name := req.Name
	if req.Kind.Kind == "Pod" {
		name = name + "-pod"
	}

	return strings.ToLower(name + "-ibm-application-gateway-sidecar-pod")
}

/*
 * Function retrieves the base configmap name.
 */
func getWebhookConfigMapName(req *v1beta1.AdmissionRequest) (string) {

	name := req.Name
	if req.Kind.Kind == "Pod" {
		name = name + "-pod"
	}

	return strings.ToLower(name + "-ibm-application-gateway-sidecar-configmap")
}

/*
 * Function creates a service template ready to be created in K8s.
 */
func newService(annots map[string]string, req *v1beta1.AdmissionRequest) *corev1.Service {

	log.Info("WebhookServer : newService")
	
	name := getServiceName(req)

	port, err := strconv.Atoi(annots[servPort])
	if err != nil {
	    port = 30443
	}

	labels := map[string]string{
		"app": req.Name,
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:      name,
			Namespace: req.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec {
			Ports: []corev1.ServicePort {
				{
					Port: 8443,
					NodePort: int32(port),
					Protocol: "TCP",
					Name: name,
				},
			},
			Type: "NodePort",
			Selector: map[string]string {
				"app": req.Name,
			},
		},
	}
}

/*
 * Function creates patches to add the IAG container and Volume definition to the existing spec.
 */
func addIAGContainer(currVolumes []corev1.Volume, annots map[string]string, containers []corev1.Container, 
	basePath string, cmName string, req *v1beta1.AdmissionRequest, update bool, configChanged bool) (patch []patchOperation, err error) {

	log.Info("WebhookServer : addIAGContainer")

	imageLocation := annots[imageAnnot]
	if imageLocation == "" {
	    return nil, fmt.Errorf("No IBM Application Gateway image has been specified.")
	}

	imagePullPolicyStr := annots["ibm-application-gateway.security.ibm.com/deployment.imagePullPolicy"]
	if imagePullPolicyStr == "" {
		imagePullPolicyStr = "IfNotPresent"
	}

	var imagePullPolicy corev1.PullPolicy
	switch strings.ToLower(imagePullPolicyStr) {
		case "never":
			imagePullPolicy = corev1.PullNever
		case "always":
			imagePullPolicy = corev1.PullAlways
		default:
			imagePullPolicy = corev1.PullIfNotPresent
	}

	// Volume only needs to be added on create or configChange
	if !update || configChanged {
		// First add the volume
		volume := corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cmName,
					},
				},
			},
		}

		handled := false
		if update {
			// Find the volume to update and replace it
			realIndex := -1
			for index, vol := range currVolumes {
				if vol.Name == volumeName {
					realIndex = index
					break
				}
			}

			// Replace if we found it, otherwise fall through to add
			if realIndex > -1 {
				patch = append(patch, patchOperation{
					Op:    "replace",
					Path:  fmt.Sprintf("%s/spec/volumes/%d", basePath, realIndex),
					Value: volume,
				})
				handled = true
			} 
		}

		if !handled {
			// First volume does not have the "/-" and must be an array
			firstVol := len(currVolumes) == 0
			path := fmt.Sprintf("%s/spec/volumes", basePath)
			var newValue interface{}
			if firstVol {
				newValue = []corev1.Volume{volume}
			} else {
				path = path + "/-"
				newValue = volume
			}

			patch = append(patch, patchOperation{
				Op:    "add",
				Path:  path,
				Value: newValue,
			})
		}
	}

	readinessCmd := "/sbin/health_check.sh"
	readinessInitDelay := 5
	readinessPeriod := 10

	livenessCmd := "/sbin/health_check.sh"
	livenessInitDelay := 120
	livenessPeriod := 20

	// Handle the specified environment settings
	var envList []corev1.EnvVar

	for key, value := range annots {
		if strings.HasPrefix(key, envPrefix) {
			realKey := strings.TrimPrefix(key, envPrefix)

			envVal := corev1.EnvVar {
				Name: realKey,
				Value: value,
			}

			envList = append(envList, envVal)
		}
	}

	// Next add the container
	iagCont := corev1.Container{
     	Name: getAppName(req), 
        Image: imageLocation,
        ImagePullPolicy: imagePullPolicy,
        Ports: []corev1.ContainerPort{
        	{
                ContainerPort: 80,
            },
        },
        VolumeMounts: []corev1.VolumeMount{
        	{
				Name:      "ibm-application-gateway-config",
				MountPath: "/var/iag/config",
			},
		},
		Env: envList,
		ReadinessProbe: &corev1.Probe {
			InitialDelaySeconds: int32(readinessInitDelay),
			PeriodSeconds: int32(readinessPeriod),
			Handler: corev1.Handler {
				Exec: &corev1.ExecAction {
					Command: []string{
						readinessCmd,
					},
				},
			},
		},
		LivenessProbe: &corev1.Probe {
			InitialDelaySeconds: int32(livenessInitDelay),
			PeriodSeconds: int32(livenessPeriod),
			Handler: corev1.Handler {
				Exec: &corev1.ExecAction {
					Command: []string{
						livenessCmd,
					},
				},
			},
		},
	}

	handled := false
	if update {

		realIndex := -1

		// Find the container to update and replace it
		for index, cont := range containers {
			if cont.Name == iagCont.Name {
				realIndex = index
				break
			}
		}

		if realIndex > -1 {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  fmt.Sprintf("%s/spec/containers/%d", basePath, realIndex),
				Value: iagCont,
			})

			handled = true
		}
	}

	if !handled {
		// First container does not have the "/-" and must be an array
		firstCont := len(containers) == 0
		cpath := fmt.Sprintf("%s/spec/containers", basePath)
		var newCont interface{}
		if firstCont {
			newCont = []corev1.Container{iagCont}
		} else {
			cpath = cpath + "/-"
			newCont = iagCont
		}

		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  cpath,
			Value: newCont,
		})
	}

	return patch, nil
}

/*
 * Function creates a patch to add any new annotations to the current annotations.
 */
func addAnnotations(currAnnots map[string]string, newAnnots map[string]string) (patch []patchOperation) {

	log.Info("WebhookServer : addAnnotations")

	oper := "replace"
	if currAnnots == nil {
		currAnnots = map[string]string{}
		oper = "add"
	}

	for key, value := range newAnnots {
		if value == "" {
			// Delete it
			delete(currAnnots, key)
		} else {
			currAnnots[key] = value
		}
	}

	patch = append(patch, patchOperation{
		Op:   oper,
		Path: "/metadata/annotations",
		Value: currAnnots,
	})

	return patch
}

/*
 * Function creates the IAG configmap and service plus the patches to mutate the target resource.
 */
func createObjects(whsvr *WebhookServer, volumes []corev1.Volume, annots map[string]string, containers []corev1.Container, 
	               basePath string, req *v1beta1.AdmissionRequest) ([]byte, error) {

	log.Info("WebhookServer : createObjects")

	var patch []patchOperation

	// First do some validation on the YAML annotations to try and make sure it won't fail part way through
	errVal, configElements := validateAnnotations(annots)
	if errVal != nil {
		return nil, errVal
	}

	var sName string
	var cmName string
	var err error

	// Next create the IAG service if the port has been specified
	if annots[servPort] != "" {
		sName, err = addIAGService(whsvr, annots, req)
		if err != nil {
			return nil, err
		}
	}

	// Next create the master config map
	cmName, err = createIAGConfig(whsvr, req, configElements, false, "")
	if err != nil {
		// Cleanup the service that was created before failure
		deleteService(whsvr, req, sName)
		return nil, err
	}

	// Create the IAG container patch
	patchOps, err := addIAGContainer(volumes, annots, containers, basePath, cmName, req, false, true)
	if err != nil {

		// Cleanup the service and configmap that was created before failure
		deleteService(whsvr, req, sName)
		deleteConfigMap(whsvr, req, cmName)

		return nil, err
	}
	patch = append(patch, patchOps...)

	// Add the annotations
	newAnnotations := make(map[string]string)
	newAnnotations[servAnnot] = sName
	newAnnotations[cmAnnot] = cmName
	patchOps = addAnnotations(annots, newAnnotations) 
	patch = append(patch, patchOps...)

	return json.Marshal(patch)
}

/*
 * Function updates the IAG configmap and service plus the patches to mutate the target resource.
 */
func updateObjects(whsvr *WebhookServer, volumes []corev1.Volume, annots map[string]string, containers []corev1.Container, 
	               basePath string, req *v1beta1.AdmissionRequest, annotationChanges []string) ([]byte, error) {

	log.Info("WebhookServer : updateObjects")

	var patch []patchOperation

	// First do some validation on the YAML annotations to try and make sure it won't fail part way through
	errVal, configElements := validateAnnotations(annots)
	if errVal != nil {
		return nil, errVal
	}

	var updateContainer bool = false
	var updateConfig bool = false
	var updateService bool = false

	for _, annot := range annotationChanges {
		if strings.HasPrefix(annot, confPrefix) {
			updateConfig = true
		} else if strings.HasPrefix(annot, servPort) {
			updateService = true
		} else if strings.HasPrefix(annot, imageAnnot) || strings.HasPrefix(annot, envPrefix) {
			// Note: in a running pod, image is the only thing that can be updated
			updateContainer = true
		}
	}

	var sName string
	cmName := annots[cmAnnot]
	var err error

	// Update the IAG service if required
	if updateService {
		sName, err = updateIAGService(whsvr, annots, req)
		if err != nil {
			return nil, err
		}
	}

	// Next create the master config map
	if updateConfig {
		cmName, err = createIAGConfig(whsvr, req, configElements, true, cmName)
		if err != nil {
			return nil, err
		}
	}

	// First create the IAG container patch
	if updateContainer {
		patchOps, err := addIAGContainer(volumes, annots, containers, basePath, cmName, req, true, updateConfig)
		if err != nil {
			return nil, err
		}
		patch = append(patch, patchOps...)
	}

	// Add the annotations
	newAnnotations := make(map[string]string)
	if updateService {
		newAnnotations[servAnnot] = sName
	}
	if updateConfig {
		newAnnotations[cmAnnot] = cmName
	}
	if updateService || updateConfig {
		patchOps := addAnnotations(annots, newAnnotations) 
		patch = append(patch, patchOps...)
	}

	return json.Marshal(patch)
}

/*
 * Function deletes the IAG service.
 */
func deleteService(whsvr *WebhookServer, req *v1beta1.AdmissionRequest, serviceName string) error {

	log.Info("WebhookServer: deleteService")

	// Check if this Service already exists
	foundSvc := &corev1.Service{}
	err := whsvr.client.Get(context.TODO(), types.NamespacedName{Name: serviceName, Namespace: req.Namespace}, foundSvc)
	if err == nil {
		// No error so must have been found
		err = whsvr.client.Delete(context.TODO(), foundSvc)
        if err != nil {
            log.Error(err, "failed to delete the service")
            return err
        }
	} else { 
		if errors.IsNotFound(err) {
			log.Info("Service did not exist")
			// No op. Does not exist so ignore
		} else {
			log.Error(err, "Encountered an error while attempting to delete the service")
			return err
		}
	}

	return nil
}

/*
 * Function deletes the IAG configmap.
 */
func deleteConfigMap(whsvr *WebhookServer, req *v1beta1.AdmissionRequest, configMapName string) error {
	
	log.Info("WebhookServer: deleteConfigMap")

	// Check if this Service already exists
	foundCM := &corev1.ConfigMap{}
	err := whsvr.client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: req.Namespace}, foundCM)
	if err == nil {
		// No error so must have been found
		err = whsvr.client.Delete(context.TODO(), foundCM)
        if err != nil {
            log.Error(err, "failed to delete the config map")
            return err
        }
	} else { 
		if errors.IsNotFound(err) {
			log.Info("Config Map did not exist")
			// No op. Does not exist so ignore
		} else {
			log.Error(err, "Encountered an error while attempting to delete the config map")
			return err
		}
	}

	return nil
}

/*
 * Function handles a mutate create operation.
 */
func (whsvr *WebhookServer) mutateCreate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	
	log.Info("WebhookServer: mutateCreate")

	req := ar.Request

	switch req.Kind.Kind {
		case "Pod":
			return whsvr.mutateCreatePod(ar)
		case "Deployment":
			return whsvr.mutateCreateDeployment(ar)
		default:
			return &v1beta1.AdmissionResponse{
				Allowed: true,
			}
	}
}

/*
 * Function handles a mutate create operation on a deployment resource.
 */
func (whsvr *WebhookServer) mutateCreateDeployment(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	
	log.Info("WebhookServer: mutateCreateDeployment")

	req := ar.Request

	var depl appsv1.Deployment
	if err := json.Unmarshal(req.Object.Raw, &depl); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	// For a deployment we don't want to handle the generated pods
	if req.Name == "" {
		log.Info(fmt.Sprintf("Skipping mutation for %s/%s due to no deployment name", depl.Namespace, depl.Name))
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	log.Info(fmt.Sprintf("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, depl.Name, req.UID, req.Operation, req.UserInfo))

	// determine whether to perform mutation
	mutReq, _ := mutationRequired(whsvr, ignoredNamespaces, &depl.ObjectMeta, false, req.Namespace, req.Name, false)
	if !mutReq {
		log.Info(fmt.Sprintf("Skipping mutation for %s/%s due to policy check", depl.Namespace, depl.Name))
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	patchBytes, err := createObjects(whsvr, depl.Spec.Template.Spec.Volumes, depl.Annotations, depl.Spec.Template.Spec.Containers, "/spec/template", req)
	if err != nil {
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.Info("AdmissionResponse: patch=" + string(patchBytes))
	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

/*
 * Function handles a mutate create operation on a POD resource.
 */
func (whsvr *WebhookServer) mutateCreatePod(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	
	log.Info("WebhookServer: MutateCreatePod")

	req := ar.Request

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	// For a deployment we don't want to handle the generated pods
	if req.Name == "" {
		log.Info(fmt.Sprintf("Skipping mutation for %s/%s due to no pod name", pod.Namespace, pod.Name))
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	log.Info(fmt.Sprintf("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo))

	// determine whether to perform mutation
	mutReq, _ := mutationRequired(whsvr, ignoredNamespaces, &pod.ObjectMeta, false, req.Namespace, req.Name, true)
	if !mutReq {
		log.Info(fmt.Sprintf("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name))
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	patchBytes, err := createObjects(whsvr, pod.Spec.Volumes, pod.Annotations, pod.Spec.Containers, "", req)
	if err != nil {
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.Info("AdmissionResponse: patch=" + string(patchBytes))
	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

/*
 * Function handles a mutate update operation.
 */
func (whsvr *WebhookServer) mutateUpdate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	
	log.Info("WebhookServer: MutateUpdate")

	req := ar.Request

	switch req.Kind.Kind {
		case "Pod":
			return whsvr.mutateUpdatePod(ar)
		case "Deployment":
			return whsvr.mutateUpdateDeployment(ar)
		default:
			return &v1beta1.AdmissionResponse{
				Allowed: true,
			}
	}
}

/*
 * Function handles a mutate update operation on a DEPLOYMENT resource.
 */
func (whsvr *WebhookServer) mutateUpdateDeployment(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	
	log.Info("WebhookServer: MutateUpdateDeployment")

	req := ar.Request

	var depl appsv1.Deployment
	if err := json.Unmarshal(req.Object.Raw, &depl); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.Info(fmt.Sprintf("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, depl.Name, req.UID, req.Operation, req.UserInfo))

	// determine whether to perform mutation
	mutReq, annotationChanges := mutationRequired(whsvr, ignoredNamespaces, &depl.ObjectMeta, true, req.Namespace, req.Name, false)
	if !mutReq {
		log.Info(fmt.Sprintf("Skipping mutation for %s/%s due to policy check", depl.Namespace, depl.Name))
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	log.Info(fmt.Sprintf("Mutate required for changes : %v", annotationChanges))

	patchBytes, err := updateObjects(whsvr, depl.Spec.Template.Spec.Volumes, depl.Annotations, depl.Spec.Template.Spec.Containers, "/spec/template", req, annotationChanges)
	if err != nil {
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.Info("AdmissionResponse: patch=" + string(patchBytes))
	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

/*
 * Function handles a mutate update operation on a POD resource.
 */
func (whsvr *WebhookServer) mutateUpdatePod(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	
	log.Info("WebhookServer: MutateUpdatePod")

	req := ar.Request

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.Info(fmt.Sprintf("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo))

	// determine whether to perform mutation
	mutReq, annotationChanges := mutationRequired(whsvr, ignoredNamespaces, &pod.ObjectMeta, true, req.Namespace, req.Name, true)
	if !mutReq {
		log.Info(fmt.Sprintf("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name))
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	log.Info(fmt.Sprintf("Mutate required for changes : %v", annotationChanges))

	patchBytes, err := updateObjects(whsvr, pod.Spec.Volumes, pod.Annotations, pod.Spec.Containers, "", req, annotationChanges)
	if err != nil {
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.Info("AdmissionResponse: patch=" + string(patchBytes))
	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

/*
 * Function handles a mutate delete operation.
 */
func (whsvr *WebhookServer) mutateDelete(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	
	log.Info("WebhookServer: mutateDelete")

	req := ar.Request

	switch req.Kind.Kind {
		case "Pod":
			return whsvr.mutateDeletePod(ar)
		case "Deployment":
			return whsvr.mutateDeleteDeployment(ar)
		default:
			return &v1beta1.AdmissionResponse{
				Allowed: true,
			}
	}
}

/*
 * Function handles a mutate delete operation on a deployment resource.
 */
func (whsvr *WebhookServer) mutateDeleteDeployment(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	
	log.Info("WebhookServer: mutateDeleteDeployment")

	req := ar.Request

	var depl appsv1.Deployment
	if err := json.Unmarshal(req.OldObject.Raw, &depl); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	// determine whether to perform mutation
	mutReq, _ := mutationRequired(whsvr, ignoredNamespaces, &depl.ObjectMeta, false, req.Namespace, req.Name, true)
	if !mutReq {
		log.Info(fmt.Sprintf("Skipping mutation for %s/%s due to policy check", depl.Namespace, depl.Name))
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	annotations := depl.ObjectMeta.GetAnnotations()

	return whsvr.mutateDeleteCommon(req, annotations)
}

/*
 * Function handles a mutate delete operation on a POD resource.
 */
func (whsvr *WebhookServer) mutateDeletePod(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	
	log.Info("WebhookServer: mutateDeletePod")

	req := ar.Request

	var pod corev1.Pod
	if err := json.Unmarshal(req.OldObject.Raw, &pod); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	// determine whether to perform mutation
	mutReq, _ := mutationRequired(whsvr, ignoredNamespaces, &pod.ObjectMeta, false, req.Namespace, req.Name, true)
	if !mutReq {
		log.Info(fmt.Sprintf("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name))
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	annotations := pod.ObjectMeta.GetAnnotations()

	return whsvr.mutateDeleteCommon(req, annotations)
}

/*
 * Function handles the common parts of a mutate delete operation.
 */
func (whsvr *WebhookServer) mutateDeleteCommon(req *v1beta1.AdmissionRequest, annots map[string]string) *v1beta1.AdmissionResponse {	

	log.Info("WebhookServer: mutateDeleteCommon")

	sName := annots[servAnnot]
	cmName := annots[cmAnnot]

	deleteService(whsvr, req, sName)
	deleteConfigMap(whsvr, req, cmName)

	return &v1beta1.AdmissionResponse{
			Allowed: true,
	}
}

/*
 * Function handles a mutate request.
 */
func (whsvr *WebhookServer) mutate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	
	log.Info("WebhookServer: mutate")

	req := ar.Request

	operation := req.Operation

	switch operation {
		case "DELETE":
			return whsvr.mutateDelete(ar)
		case "UPDATE":
			return whsvr.mutateUpdate(ar)
		case "CREATE":
			return whsvr.mutateCreate(ar)
		default:
			// We don't do anything for any other ops
			return &v1beta1.AdmissionResponse{
				Allowed: true,
			}
	}
}

/*
 * Function handles a serve request.
 */
func (whsvr *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {

	log.Info("WebhookServer: serve")

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		log.Info("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		log.Info(fmt.Sprintf("Content-Type=%s, expect application/json", contentType))
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		log.Error(err, "Can't decode body")
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = whsvr.mutate(&ar)
	}

	admissionReview := v1beta1.AdmissionReview{}
	admissionReview.APIVersion = "admission.k8s.io/v1" // Set the version to v1
	admissionReview.Kind = "AdmissionReview"
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)

	if err != nil {
		log.Error(err, "Can't encode response")
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	log.Info("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		log.Error(err, "Can't write response")
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}
