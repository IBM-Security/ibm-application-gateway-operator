/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package controllers

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	ibmv1 "github.com/ibm-security/ibm-application-gateway-operator/api/v1"
)

type IAGHeader struct {
	Name      string
	Type      string
	Value     string
	SecretKey string
}

type IAGOidcReg struct {
	DiscoveryEndpoint string
	Secret            string
	PostData          []IAGPostData
}

type IAGPostData struct {
	Name   string
	Value  string
	Values []string
}

type DiscoveryData struct {
	Registration_endpoint string
	Token_endpoint        string
}

type AccessTokenStruct struct {
	Access_token string
}

type ClientDataStruct struct {
	Client_id     string
	Client_secret string
}

const (
	configMapLabelKey     = "ibm-application-gateway.operator.security.ibm.com/configMap"
	configMapMasterKey    = "config.yaml"
	configVersionLabelKey = "ibm-application-gateway.operator.security.ibm.com/configVersion"
	langLabelKey          = "ibm-application-gateway.operator.security.ibm.com/lang"
)

// Logger
var log = logf.Log.WithName("controller_ibmapplicationgateway")

// Blank assignment to verify that IBMApplicationGatewayReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &IBMApplicationGatewayReconciler{}

// IBMApplicationGatewayReconciler reconciles a IBMApplicationGateway object
type IBMApplicationGatewayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	record.EventRecorder
	Leader string
}

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ibm.com,resources=ibmapplicationgateways,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ibm.com,resources=ibmapplicationgateways/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ibm.com,resources=ibmapplicationgateways/finalizers,verbs=update

/*
 * Reconcile reads that state of the cluster for a IBMApplicationGateway object and makes changes based on the state read
 * and what is in the IBMApplicationGateway.Spec
 * Note:
 * The Controller will requeue the Request to be processed again if the returned error is non-nil or
 * Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
 */
func (r *IBMApplicationGatewayReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	// Fetch the IBMApplicationGateway instance using the changed namespace object
	instance := &ibmv1.IBMApplicationGateway{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)

	if err != nil {

		// It was not an IBMApplicationGateway object change
		// so check to see if it was a change to a config map
		// that an IBMApplicationGateway object is referencing
		configMap := &corev1.ConfigMap{}
		err = r.Client.Get(context.TODO(), request.NamespacedName, configMap)

		if err == nil {
			// Its a config map, check to see if we need to do anything
			err = handleConfigMapChange(r, instance, request)
			if err != nil {
				reqLogger.Error(err, "Failed to update custom objects for config map change.")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		} else {

			if errors.IsNotFound(err) {
				// Request object not found, could have been deleted after reconcile request.
				// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
				// Return and don't requeue
				return ctrl.Result{}, nil
			}
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
	} else {
		reqLogger.Info("Reconciling IBMApplicationGateway")

		// Its an IBMApplicationGateway object that has changed
		// First check to see if the deployment exists for this custom resource
		dply := &appsv1.Deployment{}
		errD := r.Client.Get(context.TODO(), request.NamespacedName, dply)

		// Get the current config map version (update if necessary)
		cmVersion := ""
		cmName := ""
		cmName, cmVersion, err = createNewConfigMap(r, instance, request, dply)
		if err != nil || cmVersion == "" {
			reqLogger.Error(err, "Failed to handle the config map.")
			return manageError(r, instance, err)
		}

		// If the deplyment did not exist then create it
		if errD != nil {
			if errors.IsNotFound(errD) {

				// Need to create it
				reqLogger.Info("Creating a new deployment.")
				_, errD = createNewDeployment(r, instance, request, cmVersion, cmName)
				if errD != nil {
					reqLogger.Error(errD, "Failed to create the new deployment.")
					return manageError(r, instance, errD)
				}

				return ctrl.Result{}, nil
			}
		} else {

			// Deployment exists
			// Next check to make sure the replica count is correct
			if instance.Spec.Replicas != *dply.Spec.Replicas {
				dply.Spec.Replicas = &instance.Spec.Replicas

				reqLogger.Info("Updating deployment replica count.")
				err := r.Client.Update(context.TODO(), dply)
				if err != nil {
					reqLogger.Error(err, "Failed to update the deployment.")
					return manageError(r, instance, err)
				}

				// Update was successful
				return ctrl.Result{}, nil
			} else {

				// Replicas are correct so check other deployment options are up to date
				updateReq := false
				changeCause := ""

				// Config version
				if dply.Spec.Template.Labels[configVersionLabelKey] != cmVersion {
					updateReq = true
					changeCause = "Configuration change"
					reqLogger.Info(changeCause)
				}

				// Language
				if dply.Spec.Template.Labels[langLabelKey] != instance.Spec.Deployment.Lang {
					updateReq = true
					if changeCause == "" {
						changeCause = fmt.Sprintf("Language changed from %s to %s", dply.Spec.Template.Labels[langLabelKey], instance.Spec.Deployment.Lang)
					} else {
						changeCause = changeCause + ", language change"
					}
					reqLogger.Info(changeCause)
				}

				// Service account
				if dply.Spec.Template.Spec.ServiceAccountName != instance.Spec.Deployment.ServiceAccountName {
					updateReq = true
					if changeCause == "" {
						changeCause = fmt.Sprintf("Service account changed from %s to %s", dply.Spec.Template.Spec.ServiceAccountName, instance.Spec.Deployment.ServiceAccountName)
					} else {
						changeCause = changeCause + ", service account change"
					}
					reqLogger.Info(changeCause)
				}

				// Image location
				if dply.Spec.Template.Spec.Containers[0].Image != instance.Spec.Deployment.ImageLocation {
					updateReq = true
					if changeCause == "" {
						changeCause = fmt.Sprintf("Image changed from %s to %s", dply.Spec.Template.Spec.Containers[0].Image, instance.Spec.Deployment.ImageLocation)
					} else {
						changeCause = changeCause + ", image change"
					}
					reqLogger.Info(changeCause)
				}

				// Make the changes and update if required
				if updateReq == true {

					reqLogger.Info("Updating deployment due to " + changeCause)

					// Set the new values in the deployment spec
					dply.Spec.Template.Labels[configVersionLabelKey] = cmVersion
					dply.Spec.Template.Labels[langLabelKey] = instance.Spec.Deployment.Lang
					dply.Spec.Template.Spec.ServiceAccountName = instance.Spec.Deployment.ServiceAccountName
					dply.Spec.Template.Spec.Containers[0].Image = instance.Spec.Deployment.ImageLocation

					// Set the new env
					dply.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
						{
							Name:  "LANG",
							Value: instance.Spec.Deployment.Lang,
						},
					}

					// Update the revision history with the reason for this change
					dply.Annotations["kubernetes.io/change-cause"] = changeCause

					// Update the deployment
					err := r.Client.Update(context.TODO(), dply)
					if err != nil {
						reqLogger.Error(err, "Failed to update the deployment.")
						return manageError(r, instance, err)
					}

					// Update was successful
					return ctrl.Result{}, nil
				}
			}
		}
	}

	return ctrl.Result{Requeue: true}, nil
}

/*
 * Function will handle a config map change. It will loop through all the deployed IAG custom objects
 * to see if they reference the modified config map. If so then a noop update will be made to that
 * object which will in turn fire a reconcile change for that object. This will result in the configuration
 * being checked and possibly updating the IAG pods if required.
 */
func handleConfigMapChange(r *IBMApplicationGatewayReconciler, instance *ibmv1.IBMApplicationGateway, request ctrl.Request) error {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	// To help improve performance we ignore our operator 'leader' configmap as
	// this is updated frequently and we know that it does not contain any
	// IAG configuration data.
	if request.Name == r.Leader {
		return nil
	}

	// Fetch the instances
	instanceList := &ibmv1.IBMApplicationGatewayList{}
	err := r.Client.List(context.TODO(),
		instanceList,
		&client.ListOptions{
			Namespace: request.Namespace,
		})

	// Update (touch) any IBMApplicationGateway custom objects that use the changed config map
	for _, inst := range instanceList.Items {
		for _, entry := range inst.Spec.Configuration {
			if entry.Type == "configmap" {
				mapName := entry.Name

				if mapName == request.Name {
					reqLogger.Info("Handle configmap change : updating")

					iagNamespaceName := request.NamespacedName
					iagNamespaceName.Name = inst.Name

					err = r.Client.Get(context.TODO(), iagNamespaceName, instance)
					if err != nil {
						return err
					}

					err = r.Client.Status().Update(context.TODO(), instance)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

/*
 * Function reads the configured config locations from the custom object yaml and sequentially
 * merges each of them to produce a single configuration string in YAML format.
 */
func getMergedConfig(r *IBMApplicationGatewayReconciler, instance *ibmv1.IBMApplicationGateway, request ctrl.Request) (string, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Merging IBMApplicationGateway config")

	master := make(map[string]interface{})
	var err error

	var oidcRegs []ibmv1.IBMApplicationGatewayConfiguration

	for _, entry := range instance.Spec.Configuration {

		if entry.Type == "literal" {
			litConfig := entry.Value

			master, err = handleYamlDataMerge(litConfig, master)
			if err != nil {
				return "", err
			}

		} else if entry.Type == "configmap" {
			cmName := entry.Name
			cmDataKey := entry.DataKey

			if cmName == "" {
				return "", fmt.Errorf("Configuration configmap entry is missing the Name.")
			}
			if cmDataKey == "" {
				return "", fmt.Errorf("Configuration configmap entry is missing the DataKey.")
			}

			// Fetch the config map
			configMapFound := &corev1.ConfigMap{}
			err := r.Client.Get(context.TODO(), types.NamespacedName{Name: cmName, Namespace: instance.Namespace}, configMapFound)
			if err != nil {
				return "", err
			}

			// Get the config map data pointed at by the data key
			cmData := configMapFound.Data[cmDataKey]

			master, err = handleYamlDataMerge(cmData, master)
			if err != nil {
				return "", err
			}
		} else if entry.Type == "web" {

			webUrl := entry.Url
			headers := entry.Headers

			var iagHeaders []IAGHeader

			for _, header := range headers {
				var currHdr IAGHeader
				currHdr.Name = header.Name
				currHdr.Type = header.Type
				currHdr.Value = header.Value
				currHdr.SecretKey = header.SecretKey

				iagHeaders = append(iagHeaders, currHdr)
			}

			master, err = handleWebEntryMerge(r.Client, request.NamespacedName, webUrl, iagHeaders, master)
			if err != nil {
				reqLogger.Error(err, "Error encountered while attempting to merge the web config.")
				return "", err
			}
		} else if entry.Type == "oidc_registration" {
			oidcRegs = append(oidcRegs, entry)

			// Validate that there is no more than one OIDC registration
			if len(oidcRegs) > 1 {
				return "", fmt.Errorf("Only a single oidc_registration configuration source may be specified.")
			}
		}
	}

	// Handle the OIDC registration if one exists
	for _, entry := range oidcRegs {

		// Make sure its the correct type
		if entry.Type == "oidc_registration" {

			// Convert to the required struct
			var oidcReg IAGOidcReg
			oidcReg.DiscoveryEndpoint = entry.DiscoveryEndpoint
			oidcReg.Secret = entry.Secret

			// Add Post data to the new struct
			var postData []IAGPostData
			for _, elem := range entry.PostData {
				var currPd IAGPostData
				currPd.Name = elem.Name
				currPd.Value = elem.Value
				currPd.Values = elem.Values

				postData = append(postData, currPd)
			}
			oidcReg.PostData = postData

			// Handle the registration and merge
			master, err = handleOidcEntryMerge(r.Client, oidcReg, instance.Namespace, master)
			if err != nil {
				reqLogger.Error(err, "Error encountered while attempting to register a new OIDC client.")
				return "", err
			}

			// Make sure only 1 registered
			break
		}
	}

	// Marshal the object to a yaml byte array
	masterYaml, err := yaml.Marshal(validateStringKeysFromString(master))
	if err != nil {
		reqLogger.Error(err, "failed to marshal the YAML master configuration.")
		return "", err
	}

	// Return the string representation of the merged config
	return string(masterYaml), nil
}

/*
 * Handle dynamic client registration and merge OIDC identity into the current master config.
 */
func handleOidcEntryMerge(rclient client.Client, entry IAGOidcReg,
	ns string, master map[string]interface{}) (map[string]interface{}, error) {

	logger := log.WithName("handleOidcEntryMerge")
	logger.Info("Entry")

	// Secret is mandatory
	if entry.Secret == "" {
		return master, fmt.Errorf("The OIDC registration configuration source is missing the secret name.")
	}

	// Register the client (if necessary)
	err := handleOidcRegistration(&entry, rclient, ns)
	if err != nil {
		logger.Error(err, "Failed to handle the OIDC registration.")
		return master, err
	}

	// Now that the client has been registered, add the oidc identity settings.
	clientIdStr := "secret:" + entry.Secret + "/client_id"
	clientSecretStr := "secret:" + entry.Secret + "/client_secret"

	// If the identity/oidc YAML already exists then update the discoveryURL and client id/secret
	oidcUpdated := false
	if master["identity"] != nil {

		// Make sure the type is correct
		switch v := master["identity"].(type) {
		case map[interface{}]interface{}:
			masterIdentity := convertInterfaceKeysToString(v)
			if masterIdentity["oidc"] != nil {

				// Make sure the type is correct
				switch v2 := masterIdentity["oidc"].(type) {
				case map[interface{}]interface{}:
					masterOidc := convertInterfaceKeysToString(v2)

					// Update the values
					masterOidc["discovery_endpoint"] = entry.DiscoveryEndpoint
					masterOidc["client_id"] = clientIdStr
					masterOidc["client_secret"] = clientSecretStr

					// Set the master maps
					masterIdentity["oidc"] = masterOidc
					master["identity"] = masterIdentity

					// Flag as handled
					oidcUpdated = true
				}
			}
		}
	}

	// If its not already handled, add the OIDC identity
	if !oidcUpdated {
		// Need to make:
		// identity:
		//   oidc:
		//     discovery_endpoint: <discovery_url>
		//     client_id: secret:<secret>/client_id
		//     client_secret: secret:<secret>/client_secret

		var oidcStr = "identity:\n" +
			"  oidc:\n" +
			"    discovery_endpoint: " + entry.DiscoveryEndpoint + "\n" +
			"    client_id: " + clientIdStr + "\n" +
			"    client_secret: " + clientSecretStr

		master, err = handleYamlDataMerge(oidcStr, master)
		if err != nil {
			return master, err
		}
	}

	logger.Info("Exit")

	return master, nil
}

/*
 * Merge a web config source into the current master config.
 */
func handleWebEntryMerge(rclient client.Client, nsn types.NamespacedName,
	webUrl string, headers []IAGHeader, master map[string]interface{}) (map[string]interface{}, error) {

	if webUrl == "" {
		return nil, fmt.Errorf("Configuration web entry is missing the Url.")
	}

	log.V(1).Info("Retrieving config from " + webUrl)

	// Get the yaml from the given url
	client := &http.Client{}

	req, err := http.NewRequest("GET", webUrl, nil)

	// Add the headers if there are any
	for _, header := range headers {

		if header.Name == "" {
			return nil, fmt.Errorf("Configuration web header entry is missing the required name.")
		}
		if header.Value == "" {
			return nil, fmt.Errorf("Configuration web header entry is missing the required value.")
		}

		switch header.Type {
		case "literal":
			log.V(1).Info("Adding literal header : " + header.Name)
			req.Header.Add(header.Name, header.Value)
		case "secret":
			// Retrieve the header value from the secret
			secretNamespaceName := nsn
			secretNamespaceName.Name = header.Value

			secret := &corev1.Secret{}
			err = rclient.Get(context.TODO(), secretNamespaceName, secret)
			if err != nil {
				log.Error(err, "Failed to retrieve the authorization secret : "+header.Value)
				return nil, err
			} else {

				// Extract the raw secret. k8s automatically decodes it from base64
				hdrValue := string(secret.Data[header.SecretKey])

				if hdrValue != "" {
					log.V(1).Info("Adding secret header : " + header.Name)
					req.Header.Add(header.Name, hdrValue)
				} else {
					return nil, fmt.Errorf("The authorization secret : " + header.Value + " does not have the required key : " + header.SecretKey)
				}
			}
		default:
			// Invalid
			return nil, fmt.Errorf("Configuration web header entry has an invalid type : " + header.Type)
		}
	}

	// Make the request
	resp, err := client.Do(req)

	// Handle the response
	if err != nil {
		log.Error(err, "Failed to get web config : "+webUrl)
		return nil, err
	} else {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Error(err, "Failed to get web config data")
				return nil, err
			} else {
				webData := string(body)
				log.V(1).Info("Found web config " + webData)

				master, err = handleYamlDataMerge(webData, master)
				if err != nil {
					return nil, err
				}
			}
		} else {
			// Error response code
			err = fmt.Errorf("Error response from the remote config source.")
			log.Error(err, "HTTP Response Status:", fmt.Sprintf("%v", resp.StatusCode), fmt.Sprintf("%v", http.StatusText(resp.StatusCode)))
			return nil, err
		}
	}

	return master, nil
}

/**
 * This function will handle the conversion of new config data to a yaml map
 * and then merge that map with the existing master config.
 */
func handleYamlDataMerge(newConfig string, masterConfig map[string]interface{}) (map[string]interface{}, error) {

	// Unmarshal the new config data string into a Map
	var currentYaml map[string]interface{}
	err := yaml.Unmarshal([]byte(newConfig), &currentYaml)
	if err != nil {
		return nil, err
	}

	// At this point the YAML literal object is a recursed map
	// Could be map[string][string]
	// or map[string][map[string][string]]
	// or map[string][map[string][map[string][string]]]
	// etc
	// Need to recursively merge literal with master

	// Merge the current config map with the master config map
	return mergeMapsRecursive(masterConfig, currentYaml), nil
}

/*
 * Function converts a map of interface --> interface to a map of string --> interface
 */
func convertInterfaceKeysToString(inputMap map[interface{}]interface{}) map[string]interface{} {

	retVal := make(map[string]interface{})

	for key, value := range inputMap {
		strKey := fmt.Sprintf("%v", key)

		retVal[strKey] = value
	}

	return retVal
}

/*
 * Function validates that all map keys are of type string recursively.
 * This almost mirrors validateStringKeysFromInterface but golang doesn't seem to allow
 * a generic arg that can be casted.
 */
func validateStringKeysFromString(inputMap map[string]interface{}) map[string]interface{} {
	log.V(1).Info("convertMaster")

	retVal := make(map[string]interface{})

	for key, value := range inputMap {
		switch value2 := value.(type) {
		case map[string]interface{}:
			log.V(1).Info("String : " + key)
			retVal[key] = validateStringKeysFromString(value2)
		case map[interface{}]interface{}:
			log.V(1).Info("interface : " + key)
			retVal[fmt.Sprint(key)] = validateStringKeysFromInterface(value2)
		case []interface{}:
			// Handle array of interfaces
			var arry []interface{}
			for _, elem := range value2 {
				switch value3 := elem.(type) {
				case map[string]interface{}:
					arry = append(arry, validateStringKeysFromString(value3))
				case map[interface{}]interface{}:
					arry = append(arry, validateStringKeysFromInterface(value3))
				default:
					arry = append(arry, value3)
				}
			}

			retVal[fmt.Sprint(key)] = arry
		default:
			retVal[fmt.Sprint(key)] = value
		}
	}

	return retVal
}

/*
 * Function validates that all map keys are of type string recursively.
 * This almost mirrors validateStringKeysFromString but golang doesn't seem to allow
 * a generic arg that can be casted.
 */
func validateStringKeysFromInterface(inputMap map[interface{}]interface{}) map[string]interface{} {
	log.V(1).Info("convertInterfaceToString")

	retVal := make(map[string]interface{})

	for key, value := range inputMap {
		switch value2 := value.(type) {
		case map[interface{}]interface{}:
			retVal[fmt.Sprint(key)] = validateStringKeysFromInterface(value2)
		case map[string]interface{}:
			retVal[fmt.Sprint(key)] = validateStringKeysFromString(value2)
		case []interface{}:
			// Handle array of interfaces
			var arry []interface{}
			for _, elem := range value2 {
				switch value3 := elem.(type) {
				case map[string]interface{}:
					arry = append(arry, validateStringKeysFromString(value3))
				case map[interface{}]interface{}:
					arry = append(arry, validateStringKeysFromInterface(value3))
				default:
					arry = append(arry, value3)
				}
			}

			retVal[fmt.Sprint(key)] = arry
		default:
			retVal[fmt.Sprint(key)] = value
		}
	}

	return retVal
}

/*
 * Function merges to maps to a single map.
 * Entries in the 2nd map will overwrite entries in the 1st map
 */
func mergeMapsRecursive(inputMap1 map[string]interface{}, inputMap2 map[string]interface{}) map[string]interface{} {

	var retVal = inputMap1

	for key, value := range inputMap2 {
		if !reflect.DeepEqual(inputMap1[key], inputMap2[key]) {
			switch v := value.(type) {
			case map[interface{}]interface{}:
				switch v2 := inputMap1[key].(type) {
				case map[interface{}]interface{}:
					retVal[key] = mergeMapsRecursive(convertInterfaceKeysToString(v2), convertInterfaceKeysToString(v))
				default:
					retVal[key] = inputMap2[key]
				}
			case []interface{}:
				// Its an array of entries
				// All we do here is to add all new elements to the existing elements
				switch v2 := inputMap1[key].(type) {
				case []interface{}:

					// New container for all elements
					var allVals []interface{}

					// Add the existing elements
					for _, element := range v {
						allVals = append(allVals, element)
					}

					// Add the new elements
					for _, element := range v2 {
						allVals = append(allVals, element)
					}

					// Set the new array containing both
					retVal[key] = allVals
				default:
					// Not both arrays. This is not valid. Just set as 2nd value
					retVal[key] = inputMap2[key]
				}

			default:
				retVal[key] = inputMap2[key]
			}
		} else {
			retVal[key] = value
		}
	}

	return retVal
}

/*
 * Function creates a new config map from the merged definitions but does not deploy it
 */
func createNewConfigMap(r *IBMApplicationGatewayReconciler, instance *ibmv1.IBMApplicationGateway, request ctrl.Request, depl *appsv1.Deployment) (string, string, error) {
	reqLogger := log.WithValues("Request.Namespace", "request.Namespace", "Request.Name", "request.Name")

	// Check to see if the config has changed
	newData, err := getMergedConfig(r, instance, request)
	if err != nil {
		reqLogger.Error(err, "Failed to get merged config.")
		return "", "", err
	}

	configMap := newConfigMap(instance, newData)

	// Set Presentation instance as the owner and controller
	if err = controllerutil.SetControllerReference(instance, configMap, r.Scheme); err != nil {
		return "", "", err
	}

	// Check if this ConfigMap already exists
	foundMap, err := getCurrentConfigMap(r, instance, depl)
	if foundMap == nil || (err != nil && errors.IsNotFound(err)) {
		err = r.Client.Create(context.TODO(), configMap)
		if err != nil {
			return "", "", err
		}

		return configMap.Name, configMap.ResourceVersion, nil

	} else if err != nil {
		return "", "", err
	}

	if foundMap.Data[configMapMasterKey] != configMap.Data[configMapMasterKey] {
		reqLogger.Info("Config has changed so recreate the master configmap.")
		foundMap.Data[configMapMasterKey] = configMap.Data[configMapMasterKey]
		err = r.Client.Update(context.TODO(), foundMap)
		if err != nil {
			return "", "", err
		}
	}
	return foundMap.Name, foundMap.ResourceVersion, nil
}

/*
 * Function retrieves the current deployed IAG merged config map
 */
func getCurrentConfigMap(r *IBMApplicationGatewayReconciler, instance *ibmv1.IBMApplicationGateway, depl *appsv1.Deployment) (*corev1.ConfigMap, error) {

	configMapName := depl.Spec.Template.Labels[configMapLabelKey]

	if configMapName == "" {
		return nil, nil
	}

	// Get the ConfigMap
	foundMap := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: instance.Namespace}, foundMap)
	if err != nil {
		return nil, err
	}

	return foundMap, nil
}

/*
 * Function creates a new deployment
 */
func createNewDeployment(r *IBMApplicationGatewayReconciler, instance *ibmv1.IBMApplicationGateway,
	request ctrl.Request, cmVersion string, cmName string) (string, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	deployment := newDeploymentForCR(instance, cmVersion, cmName)

	err := r.Client.Create(context.TODO(), deployment)
	if err != nil {
		reqLogger.Error(err, "Failed to create a new deployment.")
		return "", err
	}

	return deployment.GetObjectMeta().GetName(), nil
}

/*
 * Function returns the template IAG pod name using the passed in IAG instance
 */
func getDeploymentName(cr *ibmv1.IBMApplicationGateway) string {
	return cr.Name
}

/*
 * Function returns the master configmap name using the passed in IAG instance
 */
func getConfigMapName(cr *ibmv1.IBMApplicationGateway) string {

	suffixName := cr.Spec.Deployment.ConfigMapSuffix
	if suffixName == "" {
		suffixName = "-config-iag-internal-generated"
	}
	return cr.Name + suffixName
}

/*
 * Function creates and returns a new IAG pod with the same name/namespace as the cr
 * Note that at this point the POD is not created in K8s. This is just a container.
 */
func newDeploymentForCR(cr *ibmv1.IBMApplicationGateway, cmVersion string, cmName string) *appsv1.Deployment {

	reqLogger := log.WithValues("Request.Namespace", "IBMApplicationGateway", "Request.Name", cr.Name)
	reqLogger.Info("newPodForCR")

	// These are the main k8s labels to use for the selector
	labelsSel := map[string]string{
		"app":     cr.Name,
		"version": "v0.1",
	}

	// Exract the deployment values from the custom resource yaml
	serviceAccountName := cr.Spec.Deployment.ServiceAccountName
	lang := cr.Spec.Deployment.Lang
	imageName := cr.Spec.Deployment.ImageLocation
	imagePullSecrets := cr.Spec.Deployment.ImagePullSecrets
	configMapName := cmName
	podName := getDeploymentName(cr)
	specPullPolicy := cr.Spec.Deployment.ImagePullPolicy

	// These are the template labels
	labelsTemp := map[string]string{
		"app":                 cr.Name,
		"version":             "v0.1",
		configVersionLabelKey: cmVersion,
		langLabelKey:          lang,
		configMapLabelKey:     cmName,
	}

	var imagePullPolicy corev1.PullPolicy
	switch strings.ToLower(specPullPolicy) {
	case "never":
		imagePullPolicy = corev1.PullNever
	case "always":
		imagePullPolicy = corev1.PullAlways
	default:
		imagePullPolicy = corev1.PullIfNotPresent
	}

	// Get the readiness settings
	readinessProbe := cr.Spec.Deployment.ReadinessProbe
	readinessCmd := readinessProbe.Command
	readinessInitDelay := readinessProbe.InitDelay
	readinessPeriod := readinessProbe.Period
	readinessFailureThres := readinessProbe.FailureThreshold
	readinessSuccessThres := readinessProbe.SuccessThreshold
	readinessTimeoutSeconds := readinessProbe.TimeoutSeconds

	if readinessCmd == "" {
		readinessCmd = "/sbin/health_check.sh"
	}
	if readinessInitDelay < 0 {
		readinessInitDelay = 0
	}
	if readinessPeriod < 1 {
		readinessPeriod = 10
	}
	if readinessFailureThres < 1 {
		readinessFailureThres = 3
	}
	if readinessSuccessThres < 1 {
		readinessSuccessThres = 1
	}
	if readinessTimeoutSeconds < 1 {
		readinessTimeoutSeconds = 1
	}

	// Get the liveness settings
	livenessProbe := cr.Spec.Deployment.LivenessProbe
	livenessCmd := livenessProbe.Command
	livenessInitDelay := livenessProbe.InitDelay
	livenessPeriod := livenessProbe.Period
	livenessFailureThres := livenessProbe.FailureThreshold
	livenessSuccessThres := livenessProbe.SuccessThreshold
	livenessTimeoutSeconds := livenessProbe.TimeoutSeconds

	if livenessCmd == "" {
		livenessCmd = "/sbin/health_check.sh"
	}
	if livenessInitDelay < 0 {
		livenessInitDelay = 0
	}
	if livenessPeriod < 1 {
		livenessPeriod = 10
	}
	if livenessFailureThres < 1 {
		livenessFailureThres = 3
	}
	if livenessSuccessThres != 1 {
		livenessSuccessThres = 1
	}
	if livenessTimeoutSeconds < 1 {
		livenessTimeoutSeconds = 1
	}

	// Get all of the secrets
	var ipSecrets []corev1.LocalObjectReference
	for _, secret := range imagePullSecrets {
		secObj := corev1.LocalObjectReference{
			Name: secret.Name,
		}
		ipSecrets = append(ipSecrets, secObj)
	}

	annotations := make(map[string]string)
	ilmtConfig := cr.Spec.Deployment.LicenseAnnotation
	if ilmtConfig == "production" {
		annotations["productName"] = "IBM Application Gateway"
		annotations["productId"] = "7c3292bae26f4f699486bc4b8d05166c"
		annotations["productMetric"] = "PROCESSOR_VALUE_UNIT"
		annotations["productChargedContainers"] = "All"
	} else {
		reqLogger.Info("Unknown License annotation provided, falling back to production")
		annotations["productName"] = "IBM Application Gateway"
		annotations["productId"] = "7c3292bae26f4f699486bc4b8d05166c"
		annotations["productMetric"] = "PROCESSOR_VALUE_UNIT"
		annotations["productChargedContainers"] = "All"
	}
	customAnnotations := cr.Spec.Deployment.CustomAnnotations
	if customAnnotations != nil {
		for _, e := range customAnnotations {
			annotations[e.Key] = e.Value
		}
	}

	// Create the new deployment
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: cr.Namespace,
			Labels:    labelsSel,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cr, ibmv1.GroupVersion.WithKind("IBMApplicationGateway")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &cr.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsSel,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labelsTemp,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					Volumes: []corev1.Volume{
						{
							Name: "iag-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
								},
							},
						},
					},
					ImagePullSecrets: ipSecrets,
					Containers: []corev1.Container{
						{
							Name:            podName,
							Image:           imageName, // icr.io/ibmappgateway/ibm-application-gateway:19.12
							ImagePullPolicy: imagePullPolicy,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "iag-config",
									MountPath: "/var/iag/config",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "LANG",
									Value: lang,
								},
							},
							ReadinessProbe: &corev1.Probe{
								InitialDelaySeconds: readinessInitDelay,
								PeriodSeconds:       readinessPeriod,
								FailureThreshold:    readinessFailureThres,
								SuccessThreshold:    readinessSuccessThres,
								TimeoutSeconds:      readinessTimeoutSeconds,
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											readinessCmd,
										},
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								InitialDelaySeconds: livenessInitDelay,
								PeriodSeconds:       livenessPeriod,
								FailureThreshold:    livenessFailureThres,
								SuccessThreshold:    livenessSuccessThres,
								TimeoutSeconds:      livenessTimeoutSeconds,
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											livenessCmd,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

/*
 * Function creates a new master ConfigMap with the passed in data.
 * Note that at this point the POD is not created in K8s. This is just a container.
 */
func newConfigMap(cr *ibmv1.IBMApplicationGateway, newData string) *corev1.ConfigMap {

	configMapName := getConfigMapName(cr)

	return getNewConfigMap(configMapName, cr.Name, cr.Namespace, newData)
}

/*
 * Function populates a new ConfigMap object.
 */
func getNewConfigMap(configMapName string, appName string, ns string, newData string) *corev1.ConfigMap {

	labels := map[string]string{
		"app": appName,
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: configMapName,
			Namespace:    ns,
			Labels:       labels,
		},
		Data: map[string]string{
			configMapMasterKey: newData,
		},
	}
}

/*
 * Function calls the discovery endpoint to retrieve the token and registration endpoints.
 */
func getDiscoveryData(entry *IAGOidcReg, insecure bool) (DiscoveryData, error) {

	reqLogger := log.WithName("getDiscoveryData")
	reqLogger.Info("Entry")

	var retVal DiscoveryData

	// Get the registration URL
	respData, err := doRequest(entry.DiscoveryEndpoint, "GET", []byte(""), insecure, "", "", "")
	if err != nil {
		reqLogger.Error(err, "Failed to retrieve the OIDC endpoints.")
	} else {
		err = json.Unmarshal([]byte(respData), &retVal)
		if err != nil {
			reqLogger.Error(err, "Failed to unmarshal the discovery endpoints.")
		}
	}

	reqLogger.Info("Exit")

	return retVal, err
}

/*
 * Function will attempt to retrieve an access token from the OIDC OP that can be used
 * to authorize the client registration.
 */
func getAccessToken(endpoints *DiscoveryData, tokenRetrievalClientId string, tokenRetrievalClientSecret string, insecure bool, scopes string) (string, error) {

	reqLogger := log.WithName("getAccessToken")
	reqLogger.Info("Entry")

	reqLogger.Info("Token Endpoint : " + endpoints.Token_endpoint)

	if endpoints.Token_endpoint == "" {
		err := fmt.Errorf("The discovery response does not contain the token endpoint.")
		return "", err
	}

	// Remove rogue newlines from end of secret data
	tokenRetrievalClientId = strings.TrimSuffix(tokenRetrievalClientId, "\n")
	tokenRetrievalClientSecret = strings.TrimSuffix(tokenRetrievalClientSecret, "\n")

	// Build the request data
	tokenReqData := "grant_type=client_credentials&client_id=" + tokenRetrievalClientId + "&client_secret=" + tokenRetrievalClientSecret

	// Add the scopes if there were any
	if scopes != "" {
		tokenReqData += "&scope=" + scopes
	}

	// Get the access token
	respData, err := doRequest(endpoints.Token_endpoint, "POST", []byte(tokenReqData), insecure, "", "", "")
	if err != nil {
		reqLogger.Error(err, "Failed to retrieve the access token.")
		return "", err
	}

	var clientData AccessTokenStruct
	err = json.Unmarshal([]byte(respData), &clientData)
	if err != nil {
		reqLogger.Error(err, "Failed to unmarshal the access token.")
		return "", err
	}

	retVal := clientData.Access_token
	reqLogger.Info("Exit")

	return retVal, nil
}

/*
 * Function extracts the scopes from the postData into the required format.
 */
func getScopes(entry *IAGOidcReg) string {

	reqLogger := log.WithName("getScopes")
	reqLogger.Info("Entry")

	retVal := ""

	for _, dataEntry := range entry.PostData {
		if dataEntry.Name == "scopes" {
			// Handle case where they are all defined as comma separated list
			splitScopes := strings.Split(dataEntry.Value, ",")
			for _, scope := range splitScopes {

				// Add a comma if this is not the first
				if retVal != "" {
					retVal += ","
				}

				// Add the current scope
				retVal += "\"" + strings.Trim(scope, " ") + "\""
			}
		}
	}

	reqLogger.Info("Exit")
	return retVal
}

/*
 * Function will build the request data and make the HTTP call to register a new OIDC client.
 */
func registerOidcClient(endpoints *DiscoveryData, entry *IAGOidcReg, baUser string, baPwd string, token string, insecure bool) (ClientDataStruct, error) {

	reqLogger := log.WithName("registerOidcClient")
	reqLogger.Info("Entry")

	var retVal ClientDataStruct

	// Add all of the post data key values
	dataMap := make(map[string]interface{})
	for _, dataEntry := range entry.PostData {

		if dataEntry.Name == "" {
			return retVal, fmt.Errorf("The POST data entry is missing the required name field.")
		}

		// First check if its a single value
		if dataEntry.Value != "" {
			dataMap[dataEntry.Name] = dataEntry.Value
		} else {
			// Must be an array of values
			if dataEntry.Values != nil {
				dataMap[dataEntry.Name] = dataEntry.Values
			} else {
				// Invalid
				return retVal, fmt.Errorf("The POST data entry is missing the required value(s) field : " + dataEntry.Name)
			}
		}
	}

	// Build the request body
	body, err := json.Marshal(dataMap)
	if err != nil {
		reqLogger.Error(err, "Failed to marshal the POST data.")
		return retVal, err
	}

	// Register the new client
	respData, err2 := doRequest(endpoints.Registration_endpoint, "POST", body, insecure, baUser, baPwd, token)
	if err2 != nil {
		reqLogger.Error(err2, "Failed to register the new client.")
		return retVal, err2
	}

	// Retrieve the response data client ID and secret
	err2 = json.Unmarshal([]byte(respData), &retVal)
	if err2 != nil {
		reqLogger.Error(err2, "Failed to unmarshal the client data.")
		return retVal, err2
	}

	reqLogger.Info("Exit")
	return retVal, nil
}

/*
 * This function will handle an OIDC dynamic client registration configuration source.
 * The client is registered and the oidc identity configuration snippet is returned ready to
 * be merged into the master configuration.
 */
func handleOidcRegistration(entry *IAGOidcReg, rclient client.Client, ns string) error {

	reqLogger := log.WithName("handleOidcRegistration")
	reqLogger.Info("Entry")

	// Retrieve the secret
	secret := &corev1.Secret{}
	err := rclient.Get(context.TODO(), types.NamespacedName{Name: entry.Secret, Namespace: ns}, secret)
	if err != nil {
		log.Error(err, "Failed to retrieve the OIDC registration secret : "+entry.Secret)
		return err
	}

	// If client_id and client_secret are set then no need to re-register
	clientId := string(secret.Data["client_id"])
	clientSecret := string(secret.Data["client_secret"])

	// Has insecure been set
	insecure := false
	insTlsStr := string(secret.Data["insecureTLS"])
	insTlsStr = strings.TrimSuffix(insTlsStr, "\n")
	if strings.ToUpper(insTlsStr) == "TRUE" {
		insecure = true
		reqLogger.Info("Insecure TLS has been set to true")
	}

	// If the clientID and secret already exist then no need to reregister
	if clientId == "" || clientSecret == "" {

		if entry.DiscoveryEndpoint == "" {
			return fmt.Errorf("The OIDC registration configuration source is missing the discoveryEndpoint.")
		}

		// Retrieve the discovery data from the OIDC OP
		endpoints, err2 := getDiscoveryData(entry, insecure)
		if err2 != nil {
			reqLogger.Error(err2, "Failed to retrieve the discovery data.")
			return err2
		}

		// Extract the raw secret value for BA user. k8s automatically decodes it from base64
		baUser := string(secret.Data["baUsername"])
		baPwd := string(secret.Data["baPassword"])
		bearerToken := string(secret.Data["initialAccessToken"])

		// If not BA then need to get the access token
		if baUser == "" || baPwd == "" {

			// Check to see if it already exists
			if bearerToken == "" {

				tokenRetrievalClientId := string(secret.Data["tokenRetrievalClientId"])
				tokenRetrievalClientSecret := string(secret.Data["tokenRetrievalClientSecret"])

				// Get the access token
				bearerToken, err = getAccessToken(&endpoints, tokenRetrievalClientId, tokenRetrievalClientSecret, insecure, getScopes(entry))
				if err != nil {
					// Couldn't get it. This may be ok as this is not a required token for all OPs
					reqLogger.Info("Failed to retrieve an access token from the OIDC OP.")
				}
			}
		}

		// Register the new client
		var clientData ClientDataStruct
		clientData, err = registerOidcClient(&endpoints, entry, baUser, baPwd, bearerToken, insecure)
		if err != nil {
			reqLogger.Error(err, "Failed to register the new client.")
			return err
		}

		if clientData.Client_id == "" || clientData.Client_secret == "" {
			return fmt.Errorf("The OIDC registration did not return a valid client ID or secret.")
		}

		// Add the values to the secret
		secret.Data["client_id"] = []byte(clientData.Client_id)
		secret.Data["client_secret"] = []byte(clientData.Client_secret)
		err = rclient.Update(context.TODO(), secret)
		if err != nil {
			reqLogger.Error(err, "Failed to update the Kubernetes secret with the client ID and secret.")
			return err
		}
	} else {
		reqLogger.Info("Using existing clientID and secret.")
	}

	reqLogger.Info("Exit")

	// Registered with no error
	return nil
}

/*
 * Function makes an HTTP request and returns the resulting data as a string.
 */
func doRequest(url string, method string, data []byte, insecure bool, baUser string, baPwd string, bearerToken string) (string, error) {

	logger := log.WithName("doRequest")
	logger.Info("Entry " + method + " : " + url)

	// Setup the TLS certs
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	// Add service account CA to rootCAs
	if !insecure {
		cert, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt")
		if err == nil {
			rootCAs.AppendCertsFromPEM(cert)
		} else {
			logger.Info("No service account CA certificate has been set")
		}
	}

	// Create the client
	client := &http.Client{
		Timeout: time.Second * 20,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            rootCAs,
				InsecureSkipVerify: insecure,
			},
		},
	}

	var body = []byte(data)

	request, err := http.NewRequest(method, url, bytes.NewBuffer(body))

	// Set the correct content headers
	if strings.HasPrefix(string(body), "{") {
		request.Header.Set("Content-type", "application/json")
		request.Header.Set("Accept", "application/json")
	} else {
		request.Header.Set("Content-type", "application/x-www-form-urlencoded")
	}

	// Set Authorization header
	if baUser != "" && baPwd != "" {
		logger.Info("Using basic authentication")
		request.SetBasicAuth(baUser, baPwd)
	} else if bearerToken != "" {
		logger.Info("Using Bearer token authentication")
		request.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	// Make the call
	resp, err := client.Do(request)
	if resp == nil {
		logger.Error(err, "The request to the OIDC provider returned a null response.")
		return "", err
	}
	if err != nil {
		logger.Error(err, "Request failed.")
		return "", err
	}

	// Handle the response
	defer resp.Body.Close()
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error(err, "Failed to get response data.")
		return "", err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		err = fmt.Errorf("%v", resp)
		logger.Error(err, "The request to the OIDC provider failed.")
		return "", err
	}

	logger.Info("Exit")
	return string(respBytes), nil
}

/**
 * Function handles an error by adding an event to the IAG instance custom resource.
 */
func manageError(r *IBMApplicationGatewayReconciler, instance *ibmv1.IBMApplicationGateway, issue error) (ctrl.Result, error) {

	logger := log.WithName("manageError")
	logger.Info("Entry")

	instance.Status.Status = false
	r.EventRecorder.Event(instance, "Warning", "Failed", issue.Error())
	err := r.Client.Status().Update(context.Background(), instance)
	if err != nil {
		// Just log an error
		logger.Error(err, "Could not update the custom object with the error event.")
	}
	logger.Info("Exit")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IBMApplicationGatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ibmv1.IBMApplicationGateway{}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
