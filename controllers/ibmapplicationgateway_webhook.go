/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package controllers

/*****************************************************************************/

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"strconv"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1      "k8s.io/api/apps/v1"
	corev1      "k8s.io/api/core/v1"
	admissionv1 "k8s.io/api/admission/v1"
	metav1      "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*****************************************************************************/

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

type IAGConfigElement struct {
	Name string
	Type string
	DataKey string
	Value string
	Url string 
	Headers []IAGHeader
	Order  int
	DiscoveryEndpoint string
	Secret string
	PostData []IAGPostData
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

/*****************************************************************************/

// +kubebuilder:webhook:path=/mutate-v1-iag,mutating=true,failurePolicy=fail,sideEffects=None,groups=apps;core,resources=deployments;pods,verbs=create;update;delete,versions=v1,name=iag.kb.io,admissionReviewVersions={v1}

/*****************************************************************************/

/*
 * Our Webhook structure.
 */

type IBMApplicationGatewayWebhook struct {
	Client  client.Client
	decoder *admission.Decoder
}

/*
 * Function checks whether the target resoured need to be mutated
 */
func mutationRequired(whsvr *IBMApplicationGatewayWebhook, ignoredList []string, metadata *metav1.ObjectMeta, 
	                  isUpdate bool, ns string, appName string, isPod bool) (bool, []string) {

	log.V(2).Info("IBMApplicationGatewayWebhook: mutationRequired")

	var annotationChanges []string

	// skip special kubernete system namespaces
	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			log.V(2).Info(fmt.Sprintf("Skip mutation for %v for it's in special namespace:%v", metadata.Name, metadata.Namespace))
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
			err = whsvr.Client.Get(context.TODO(), types.NamespacedName{Name: appName, Namespace: ns}, obj)
			annots = obj.Annotations
		} else {
			obj := &appsv1.Deployment{}
			err = whsvr.Client.Get(context.TODO(), types.NamespacedName{Name: appName, Namespace: ns}, obj)
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

	log.V(1).Info(fmt.Sprintf("Mutation policy for %v/%v: required:%v", metadata.Namespace, metadata.Name, required))
	return required, annotationChanges
}

/*
 * Do some validation of the annotation entries to try and catch errors before changes are made.
 */
func validateAnnotations(annots map[string]string) (error, []IAGConfigElement) {

	log.V(2).Info("IBMApplicationGatewayWebhook: validateAnnotations")

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

	log.V(2).Info("IBMApplicationGatewayWebhook: getConfigElements")

	var err error

	var configElements []IAGConfigElement
	cfgAnnotations := make(map[string]string)
	configNames := make(map[string]struct{})
	hdrNames := make(map[string]struct{})
	pdNames := make(map[string]struct{})
	pdValues := make(map[string][]string)

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

			// Also save each unique postData name
			postDataKey := strings.TrimPrefix(realKey, configParts[0])

			if strings.HasPrefix(postDataKey, ".postData.") {
				postDataKey = strings.TrimPrefix(postDataKey, ".postData.")

				// Next part is the header name
				postDataParts := strings.Split(postDataKey, ".")

				pdNames[postDataParts[0]] = struct{}{}

				// Need to grab the "values" array entries here for each postdata name
				valuesKey := strings.TrimPrefix(postDataKey, postDataParts[0])
				if strings.HasPrefix(valuesKey, ".values") {
					// Check if this post data name already has values
					_, ok := pdValues[postDataParts[0]]
					if ok {
						// It does so add it to the existing list
						pdValues[postDataParts[0]] = append(pdValues[postDataParts[0]], value)
					} else {
						// Need to create a new list (array) and add it
						var newList []string
						newList = append(newList, value)
						pdValues[postDataParts[0]] = newList
					}
				}
			}
		}
	}

	oidcExists := false

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

			case "oidc_registration":

				// Error if more than one oidc_registration exists
				if oidcExists {
					return nil, fmt.Errorf("Configuration must not contain multiple oidc_registration entries") 
				}

				// Oidc registration has a discoveryEndpoint, secret and postData
				currElem.DiscoveryEndpoint = cfgAnnotations[name + ".discoveryEndpoint"]
				currElem.Secret = cfgAnnotations[name + ".secret"]

				// Get the post data
				var postData []IAGPostData

				// There can be multiple postData entries defined in the form
				// configuration.sample.postData.<name>: <vals>
				for pdName := range(pdNames) {

					// Create the postData prefix
					var pdPrefix = name + ".postData." + pdName

					var currPd IAGPostData

					// Set the postdata name and value(s)
					currPd.Name = cfgAnnotations[pdPrefix + ".name"]

					// Name is required
					if currPd.Name != "" {

						// Get the value if it exists
						currPd.Value = cfgAnnotations[pdPrefix + ".value"]

						// Check for values if value has not been specified
						if currPd.Value == "" {
							currPd.Values = pdValues[pdName]
						}
						postData = append(postData, currPd)
					}
				}

				currElem.PostData = postData
				oidcExists = true
				
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
func createIAGConfig(whsvr *IBMApplicationGatewayWebhook, req *admissionv1.AdmissionRequest, configElements []IAGConfigElement, update bool, cmName string) (string, error) {

	log.V(2).Info("IBMApplicationGatewayWebhook: createIAGConfig")

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
func mergeIAGConfig(whsvr *IBMApplicationGatewayWebhook, configElements []IAGConfigElement, ns string, req *admissionv1.AdmissionRequest, update bool, cmName string) (string, error) {

	log.V(2).Info("IBMApplicationGatewayWebhook : mergeIAGConfig")

	master := make(map[string]interface {})
	var err error

	var oidcReg IAGConfigElement
	foundOidcElem := false

	for _, element := range configElements {
		switch element.Type {
			case "configmap":
				// Handle configmap entry
				master, err = handleIAGConfigMap(whsvr, element.Name, element.DataKey, ns, master)
				if err != nil {
					log.Error(err, "Error encountered attempting to merge a config map : " + element.Name)
					return "", err
				}
			case "web":
				// Handle web entry
				master, err = handleWebEntryMerge(whsvr.Client, types.NamespacedName{Name: "dummy", Namespace: ns}, 
					                              element.Url, element.Headers, master)
				if err != nil {
					log.Error(err, "Error encountered attempting to merge a web config : " + element.Url)
					return "", err
				}

			case "oidc_registration":
				// Don't handle it here. Need to make sure the oidc registration happens last
				oidcReg = element 
				foundOidcElem = true
		}
	}

	// OIDC registration must happen last
	if foundOidcElem {

		// Build the required struct
		var iagOidcReg IAGOidcReg
		iagOidcReg.DiscoveryEndpoint = oidcReg.DiscoveryEndpoint
		iagOidcReg.Secret = oidcReg.Secret
		iagOidcReg.PostData = oidcReg.PostData

		// Handle the registration and merge
		master, err = handleOidcEntryMerge(whsvr.Client, iagOidcReg, ns, master)
		if err != nil {
			log.Error(err, "Error encountered attempting to merge OIDC registration : " + oidcReg.Name)
			return "", err
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
	err = whsvr.Client.Create(context.TODO(), configMap)
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
func handleIAGConfigMap(whsvr *IBMApplicationGatewayWebhook, configMap string, dataKey string, ns string, masterConfig map[string]interface {}) (map[string]interface {}, error) {
	
	log.V(2).Info("IBMApplicationGatewayWebhook : handleIAGConfigMap")

	// Fetch the config map
	configMapFound := &corev1.ConfigMap{}
	err := whsvr.Client.Get(context.TODO(), types.NamespacedName{Name: configMap, Namespace: ns}, configMapFound)
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
func addIAGService(whsvr *IBMApplicationGatewayWebhook, annots map[string]string, req *admissionv1.AdmissionRequest) (string, error) {

	log.V(2).Info("IBMApplicationGatewayWebhook : addIAGService")

	service := newService(annots, req)

	err := whsvr.Client.Create(context.TODO(), service)
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
func updateIAGService(whsvr *IBMApplicationGatewayWebhook, annots map[string]string, req *admissionv1.AdmissionRequest) (string, error) {

	log.V(2).Info("IBMApplicationGatewayWebhook : updateIAGService")

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
func getServiceName(req *admissionv1.AdmissionRequest) string {

	name := req.Name
	if req.Kind.Kind == "Pod" {
		name = name + "-pod"
	}

	return strings.ToLower(name + "-ibm-application-gateway-sidecar-svc")
}

/*
 * Function retrieves the base application name.
 */
func getAppName(req *admissionv1.AdmissionRequest) (string) {

	name := req.Name
	if req.Kind.Kind == "Pod" {
		name = name + "-pod"
	}

	return strings.ToLower(name + "-ibm-application-gateway-sidecar-pod")
}

/*
 * Function retrieves the base configmap name.
 */
func getWebhookConfigMapName(req *admissionv1.AdmissionRequest) (string) {

	name := req.Name
	if req.Kind.Kind == "Pod" {
		name = name + "-pod"
	}

	return strings.ToLower(name + "-ibm-application-gateway-sidecar-configmap")
}

/*
 * Function creates a service template ready to be created in K8s.
 */
func newService(annots map[string]string, req *admissionv1.AdmissionRequest) *corev1.Service {

	log.V(2).Info("IBMApplicationGatewayWebhook : newService")
	
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
	basePath string, cmName string, req *admissionv1.AdmissionRequest, update bool, configChanged bool) (patch []patchOperation, err error) {

	log.V(2).Info("IBMApplicationGatewayWebhook : addIAGContainer")

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
			ProbeHandler: corev1.ProbeHandler {
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
			ProbeHandler: corev1.ProbeHandler {
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

	log.V(2).Info("IBMApplicationGatewayWebhook : addAnnotations")

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
func createObjects(whsvr *IBMApplicationGatewayWebhook, volumes []corev1.Volume, annots map[string]string, containers []corev1.Container, 
	               basePath string, req *admissionv1.AdmissionRequest) ([]byte, error) {

	log.V(2).Info("IBMApplicationGatewayWebhook : createObjects")

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
func updateObjects(whsvr *IBMApplicationGatewayWebhook, volumes []corev1.Volume, annots map[string]string, containers []corev1.Container, 
	               basePath string, req *admissionv1.AdmissionRequest, annotationChanges []string) ([]byte, error) {

	log.V(2).Info("IBMApplicationGatewayWebhook : updateObjects")

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
func deleteService(whsvr *IBMApplicationGatewayWebhook, req *admissionv1.AdmissionRequest, serviceName string) error {

	log.V(2).Info("IBMApplicationGatewayWebhook: deleteService")

	// Check if this Service already exists
	foundSvc := &corev1.Service{}
	err := whsvr.Client.Get(context.TODO(), types.NamespacedName{Name: serviceName, Namespace: req.Namespace}, foundSvc)
	if err == nil {
		// No error so must have been found
		err = whsvr.Client.Delete(context.TODO(), foundSvc)
        if err != nil {
            log.Error(err, "failed to delete the service")
            return err
        }
	} else { 
		if errors.IsNotFound(err) {
			log.V(2).Info("Service did not exist")
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
func deleteConfigMap(whsvr *IBMApplicationGatewayWebhook, req *admissionv1.AdmissionRequest, configMapName string) error {
	
	log.V(2).Info("IBMApplicationGatewayWebhook: deleteConfigMap")

	// Check if this Service already exists
	foundCM := &corev1.ConfigMap{}
	err := whsvr.Client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: req.Namespace}, foundCM)
	if err == nil {
		// No error so must have been found
		err = whsvr.Client.Delete(context.TODO(), foundCM)
        if err != nil {
            log.Error(err, "failed to delete the config map")
            return err
        }
	} else { 
		if errors.IsNotFound(err) {
			log.V(2).Info("Config Map did not exist")
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
func (whsvr *IBMApplicationGatewayWebhook) mutateCreate(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	
	log.V(2).Info("IBMApplicationGatewayWebhook: mutateCreate")

	switch req.Kind.Kind {
		case "Pod":
			return whsvr.mutateCreatePod(req)
		case "Deployment":
			return whsvr.mutateCreateDeployment(req)
		default:
			return &admissionv1.AdmissionResponse{
				Allowed: true,
			}
	}
}

/*
 * Function handles a mutate create operation on a deployment resource.
 */
func (whsvr *IBMApplicationGatewayWebhook) mutateCreateDeployment(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	
	log.V(2).Info("IBMApplicationGatewayWebhook: mutateCreateDeployment")

	var depl appsv1.Deployment
	if err := json.Unmarshal(req.Object.Raw, &depl); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	// For a deployment we don't want to handle the generated pods
	if req.Name == "" {
		log.V(2).Info(fmt.Sprintf("Skipping mutation for %s/%s due to no deployment name", depl.Namespace, depl.Name))
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	log.V(2).Info(fmt.Sprintf("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, depl.Name, req.UID, req.Operation, req.UserInfo))

	// determine whether to perform mutation
	mutReq, _ := mutationRequired(whsvr, ignoredNamespaces, &depl.ObjectMeta, false, req.Namespace, req.Name, false)
	if !mutReq {
		log.V(2).Info(fmt.Sprintf("Skipping mutation for %s/%s due to policy check", depl.Namespace, depl.Name))
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	patchBytes, err := createObjects(whsvr, depl.Spec.Template.Spec.Volumes, depl.Annotations, depl.Spec.Template.Spec.Containers, "/spec/template", req)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.V(0).Info("AdmissionResponse: patch=" + string(patchBytes))
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

/*
 * Function handles a mutate create operation on a POD resource.
 */
func (whsvr *IBMApplicationGatewayWebhook) mutateCreatePod(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	
	log.V(2).Info("IBMApplicationGatewayWebhook: MutateCreatePod")

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	// For a deployment we don't want to handle the generated pods
	if req.Name == "" {
		log.V(2).Info(fmt.Sprintf("Skipping mutation for %s/%s due to no pod name", pod.Namespace, pod.Name))
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	log.V(2).Info(fmt.Sprintf("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo))

	// determine whether to perform mutation
	mutReq, _ := mutationRequired(whsvr, ignoredNamespaces, &pod.ObjectMeta, false, req.Namespace, req.Name, true)
	if !mutReq {
		log.V(2).Info(fmt.Sprintf("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name))
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	patchBytes, err := createObjects(whsvr, pod.Spec.Volumes, pod.Annotations, pod.Spec.Containers, "", req)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.V(0).Info("AdmissionResponse: patch=" + string(patchBytes))
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

/*
 * Function handles a mutate update operation.
 */
func (whsvr *IBMApplicationGatewayWebhook) mutateUpdate(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	
	log.V(2).Info("IBMApplicationGatewayWebhook: MutateUpdate")

	switch req.Kind.Kind {
		case "Pod":
			return whsvr.mutateUpdatePod(req)
		case "Deployment":
			return whsvr.mutateUpdateDeployment(req)
		default:
			return &admissionv1.AdmissionResponse{
				Allowed: true,
			}
	}
}

/*
 * Function handles a mutate update operation on a DEPLOYMENT resource.
 */
func (whsvr *IBMApplicationGatewayWebhook) mutateUpdateDeployment(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	
	log.V(2).Info("IBMApplicationGatewayWebhook: MutateUpdateDeployment")

	var depl appsv1.Deployment
	if err := json.Unmarshal(req.Object.Raw, &depl); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.V(2).Info(fmt.Sprintf("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, depl.Name, req.UID, req.Operation, req.UserInfo))

	// determine whether to perform mutation
	mutReq, annotationChanges := mutationRequired(whsvr, ignoredNamespaces, &depl.ObjectMeta, true, req.Namespace, req.Name, false)
	if !mutReq {
		log.V(2).Info(fmt.Sprintf("Skipping mutation for %s/%s due to policy check", depl.Namespace, depl.Name))
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	log.V(0).Info(fmt.Sprintf("Mutate required for changes : %v", annotationChanges))

	patchBytes, err := updateObjects(whsvr, depl.Spec.Template.Spec.Volumes, depl.Annotations, depl.Spec.Template.Spec.Containers, "/spec/template", req, annotationChanges)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.V(0).Info("AdmissionResponse: patch=" + string(patchBytes))
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

/*
 * Function handles a mutate update operation on a POD resource.
 */
func (whsvr *IBMApplicationGatewayWebhook) mutateUpdatePod(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	
	log.V(2).Info("IBMApplicationGatewayWebhook: MutateUpdatePod")

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.V(2).Info(fmt.Sprintf("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo))

	// determine whether to perform mutation
	mutReq, annotationChanges := mutationRequired(whsvr, ignoredNamespaces, &pod.ObjectMeta, true, req.Namespace, req.Name, true)
	if !mutReq {
		log.V(2).Info(fmt.Sprintf("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name))
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	log.V(0).Info(fmt.Sprintf("Mutate required for changes : %v", annotationChanges))

	patchBytes, err := updateObjects(whsvr, pod.Spec.Volumes, pod.Annotations, pod.Spec.Containers, "", req, annotationChanges)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.V(0).Info("AdmissionResponse: patch=" + string(patchBytes))
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

/*
 * Function handles a mutate delete operation.
 */
func (whsvr *IBMApplicationGatewayWebhook) mutateDelete(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	
	log.V(2).Info("IBMApplicationGatewayWebhook: mutateDelete")

	switch req.Kind.Kind {
		case "Pod":
			return whsvr.mutateDeletePod(req)
		case "Deployment":
			return whsvr.mutateDeleteDeployment(req)
		default:
			return &admissionv1.AdmissionResponse{
				Allowed: true,
			}
	}
}

/*
 * Function handles a mutate delete operation on a deployment resource.
 */
func (whsvr *IBMApplicationGatewayWebhook) mutateDeleteDeployment(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	
	log.V(2).Info("IBMApplicationGatewayWebhook: mutateDeleteDeployment")

	var depl appsv1.Deployment
	if err := json.Unmarshal(req.OldObject.Raw, &depl); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	// determine whether to perform mutation
	mutReq, _ := mutationRequired(whsvr, ignoredNamespaces, &depl.ObjectMeta, false, req.Namespace, req.Name, true)
	if !mutReq {
		log.V(2).Info(fmt.Sprintf("Skipping mutation for %s/%s due to policy check", depl.Namespace, depl.Name))
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	annotations := depl.ObjectMeta.GetAnnotations()

	return whsvr.mutateDeleteCommon(req, annotations)
}

/*
 * Function handles a mutate delete operation on a POD resource.
 */
func (whsvr *IBMApplicationGatewayWebhook) mutateDeletePod(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	
	log.V(2).Info("IBMApplicationGatewayWebhook: mutateDeletePod")

	var pod corev1.Pod
	if err := json.Unmarshal(req.OldObject.Raw, &pod); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	// determine whether to perform mutation
	mutReq, _ := mutationRequired(whsvr, ignoredNamespaces, &pod.ObjectMeta, false, req.Namespace, req.Name, true)
	if !mutReq {
		log.V(2).Info(fmt.Sprintf("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name))
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	annotations := pod.ObjectMeta.GetAnnotations()

	return whsvr.mutateDeleteCommon(req, annotations)
}

/*
 * Function handles the common parts of a mutate delete operation.
 */
func (whsvr *IBMApplicationGatewayWebhook) mutateDeleteCommon(req *admissionv1.AdmissionRequest, annots map[string]string) *admissionv1.AdmissionResponse {	

	log.V(2).Info("IBMApplicationGatewayWebhook: mutateDeleteCommon")

	sName := annots[servAnnot]
	cmName := annots[cmAnnot]

	deleteService(whsvr, req, sName)
	deleteConfigMap(whsvr, req, cmName)

	return &admissionv1.AdmissionResponse{
			Allowed: true,
	}
}

/*
 * Function handles a mutate request.
 */
func (whsvr *IBMApplicationGatewayWebhook) mutate(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	
	log.V(2).Info("IBMApplicationGatewayWebhook: mutate")

	operation := req.Operation

	switch operation {
		case "DELETE":
			return whsvr.mutateDelete(req)
		case "UPDATE":
			return whsvr.mutateUpdate(req)
		case "CREATE":
			return whsvr.mutateCreate(req)
		default:
			// We don't do anything for any other ops
			return &admissionv1.AdmissionResponse{
				Allowed: true,
			}
	}
}

/*****************************************************************************/

/*
 * The Handle() function is called whenever the one of our resource types is
 * modified.
 */

func (a *IBMApplicationGatewayWebhook) Handle(
			ctx context.Context, req admission.Request) admission.Response {
	log.V(2).Info("IBMApplicationGatewayWebhook: Handle")

	/*
	 * Process the request, determining if mutation is required.
	 */

	admissionResponse := a.mutate(&req.AdmissionRequest)

	/*
	 * Return the response.
	 */

	resp := admission.Response{
		AdmissionResponse: *admissionResponse,
	}

	return resp
}

/*****************************************************************************/

/*
 * The InjectDecoder function injects the decoder.
 */

func (a *IBMApplicationGatewayWebhook) InjectDecoder(d *admission.Decoder) error {
	log.V(2).Info("IBMApplicationGatewayWebhook: InjectDecoder")

	a.decoder = d

	return nil
}

