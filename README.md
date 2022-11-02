# IBM Application Gateway Operator

  * [Overview](#overview)
  * [Models](#models)
    + [Sidecar](#sidecar)
    + [Custom Resource Model](#custom-resource-model)
  * [Installation](#installation)
    + [RedHat OpenShift Environment](#redhat-openshift-environment)
      - [Procedure](#procedure)
    + [Standard Kubernetes Environment](#standard-kubernetes-environment)
        * [Procedure](#procedure-1)
  * [Usage](#usage)
    + [Dynamic Client Registration](#dynamic-client-registration)
      - [OIDC Registration Configuration Source](#oidc-registration-configuration-source)
        * [Discovery Endpoint](#discovery-endpoint)
        * [POST Data](#post-data)
        * [Secret](#secret)
      - [OIDC Client Lifecycle Management](#oidc-client-lifecycle-management)
      - [Trust OIDC OP Certificate](#trust-oidc-op-certificate)
    + [Custom Resource Model](#custom-resource-model-1)
      - [Custom Object](#custom-object)
        * [Literal Source](#literal-source)
        * [Config Map Source](#config-map-source)
        * [Web Source](#web-source)
          - [Web Configuration Updates](#web-configuration-updates)
        * [OIDC Registration Configuration Source](#oidc-registration-configuration-source-1)
      - [Custom Object changes](#custom-object-changes)
        * [Changing Replica Count](#changing-replica-count)
        * [Changing Language](#changing-language)
        * [Changing the Service Account](#changing-the-service-account)
        * [Changing the Literal Configuration](#changing-the-literal-configuration)
        * [Changing a Referenced Config Map](#changing-a-referenced-config-map)
        * [Upgrading an IBM Application Gateway instance by changing the image location](#upgrading-an-ibm-application-gateway-instance-by-changing-the-image-location)
      - [Revision History](#revision-history)
      - [Deleting an IBM Application Gateway Custom Resource](#deleting-an-ibm-application-gateway-custom-resource)
      - [Split Configuration Example](#split-configuration-example)
      - [Hello World Example](#hello-world-example)
    + [Sidecar Model](#sidecar-model)
      - [Annotations](#annotations)
        * [Deployment annotations](#deployment-annotations)
        * [Service annotations](#service-annotations)
        * [Configuration annotations](#configuration-annotations)
        * [Environment annotations](#environment-annotations)
      - [Supported RESTful operations](#supported-restful-operations)
        * [Create](#create)
        * [Update](#update)
        * [Delete](#delete)
      - [Example Deployment](#example-deployment)
      - [Hello World Example](#hello-world-example-1)
  * [Troubleshooting](#troubleshooting)

      
## Overview
[IBM Application Gateway](https://docs.verify.ibm.com/gateway) provides a containerized secure Web Reverse proxy which is designed to sit in front of your application, seamlessly adding authentication and authorization protection to your application..

For a detailed description of the IBM Application Gateway see the [Offical documentation](https://docs.verify.ibm.com/gateway).

The IBM Application Gateway operator provides lifecycle management of IBM Application Gateway instances running inside a Kubernetes container.

## Models

There are two different models supported by the IBM Application Gateway operator.   The model that should be used depends on the individual requirements for each scenario. 

### Sidecar

The [sidecar](#sidecar-model) deployment model involves defining the IAG configuration using annotations to a Kubernetes application deployment YAML. Upon deployment of the application the IBM Application Gateway operator will be invoked to create an IBM Application Gateway instance in the same pod as the application.

![Sidecar Model](docs/images/operator-sidecar.png)


The sidecar deployment of IBM Application Gateway is suitable when:

* The application to protect is a Kubernetes application
* IBM Application Gateway annotations can be added to the application deployment YAML
* A one to one mapping of application to IBM Application Gateway is desirable 

### Custom Resource Model

The [custom resource](#custom-resource-model-1) deployment model involves creating an IBM Application Gateway custom resource. The operator will handle the creation and lifecycle management of the IBM Application Gateway instance. 

![Custom Resource Model](docs/images/operator-cr.png)

The custom resource deployment of IBM Application Gateway is suitable when:

* The IBM Application Gateway configuration needs to be split across multiple sources
* There are multiple instances of the IBM Application Gateway required.  The operator will manage all instances with minimal assistance from the Kubernetes administrator.

## Installation

### RedHat OpenShift Environment

The [RedHat Operator Catalog](https://catalog.redhat.com/software/operators/search) provides a single place where Kubernetes administrators or developers can go to find existing operators that may provide the functionality that they require in an OpenShift environment. 

The information provided by the [RedHat Operator Catalog](https://catalog.redhat.com/software/operators/search) allows the Operator Lifecycle Manager (OLM) to manage the operator throughout its complete lifecycle. This includes the initial installation and subscription to the RedHat Operator Catalog such that updates to the operator can be performed automatically.

#### Procedure

To install the IBM Application Gateway operator from the RedHat Operator Catalog:

1. Log into the OpenShift console as an administrator.
2. In the left navigation column, click Operators and then OperatorHub. Type 'ibm-application-gateway-operator' in the search box, and click on the IBM Application Gateway box that appears.
![OpenShift Operator Search](docs/images/openshift-operator-search.png)
3. After reviewing the product information, click the `Install` button.
![OpenShift Operator Info](docs/images/openshift-operator-product-info.png)
4. On the 'Install Operator' page that opens, specify the cluster namespace in which to install the operator. Also click the `Automatic` radio button under Approval Strategy, to enable automatic updates of the running Operator instance without manual approval from the administrator. Click the `Install` button.
![OpenShift Operator Subscription](docs/images/openshift-operator-subscription.png)
5. Ensure that the IBM Application Gateway operator has been installed correctly by the Operator Lifecycle Manager. 
![OpenShift Operator Installed](docs/images/openshift-operator-installed.png)

At this point the Operator Lifecycle Manager has been installed into the Kubernetes cluster, the IBM Application Gateway operator has been deployed and a subscription has been created that will monitor for any updates to the operator in the RedHat Operator Catalog. The IBM Application Gateway operator is now operational and any subsequent resources which are created of the kind `IBMApplicationGateway`, or deployments with the required sidecar annotations, will result in the operator being invoked to manage the deployment.


### Standard Kubernetes Environment

In a standard (i.e. non-OpenShift) Kubernetes environment the operator can be installed and managed using the [Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager) and [OperatorHub.io](https://operatorhub.io/). 
OperatorHub.io provides a single place where Kubernetes administrators or developers can go to find existing operators that may provide the functionality that they require. 

The information provided by [OperatorHub.io](https://operatorhub.io/) allows the Operator Lifecycle Manager (OLM) to manage the operator throughout its complete lifecycle. This includes the initial installation and subscription to OperatorHub.io such that updates to the operator can be performed automatically.

##### Procedure

To install the IBM Application Gateway operator from OperatorHub.io:

1. Access the [IBM Application Gateway operator page on OperatorHub.io](https://operatorhub.io/operator/ibm-application-gateway-operator) in a browser.

2. Click the Install button on the page and follow the installation instructions.

3. Ensure that the IBM Application Gateway operator has been created by the Operator Lifecycle Manager. The phase should be set to "Succeeded". Note that this may take a few minutes.

```shell
kubectl get csv -n operators

NAME                                        DISPLAY                            VERSION   REPLACES   PHASE
ibm-application-gateway-operator.v22.02.0   IBM Application Gateway Operator   22.02.0              Succeeded
``` 

At this point the Operator Lifecycle Manager has been installed into the Kubernetes cluster, the IBM Application Gateway operator has been deployed and a subscription has been created that will monitor for any updates to the operator on OperatorHub.io. The IBM Application Gateway operator is now operational and any subsequent resources which are created of the kind `IBMApplicationGateway`, or deployments with the required sidecar annotations, will result in the operator being invoked to manage the deployment.


## Usage

### Dynamic Client Registration

The IBM Application Gateway can be configured to act as an OIDC relying party for authentication. A client must be registered with the OpenID provider (OP) such that it can be referenced in the configuration. For example:

```yaml
version: "22.07"
  identity:
    oidc:
      client_id: 11111111-2222-3333-4444-5a5a5a5a5a5a5a
      client_secret: 1a2b3c4d5e
      discovery_endpoint: https://www.test.com/mga/sps/oauth/oauth20/metadata/oidc_def
``` 

The OIDC dynamic client registration negates the need for the client to be preregistered with the OIDC provider. Instead, during the merging of the configuration sources the operator will make an HTTP call to the OIDC provider to dynamically register the new client.

#### OIDC Registration Configuration Source

A single OIDC registration definition may be specified in the custom resource, or sidecar annotations. Providing more than one definition will result in an error. The operator requires the following properties to be specified:

* Discovery Endpoint: This is the endpoint that can be used to discover the registration endpoint and token endpoint of the OIDC provider.
* POST Data: Specifies any POST data that is required to be sent as part of the registration request.
* Secret: Specifies a Kubernetes secret that may contain authorization data for the registration request. This is also the location where the resulting client ID and secret are stored upon successful registration.

The following custom resource definition illustrates how to specify the OIDC registration definition:

```yaml
apiVersion: ibm.com/v1
kind: IBMApplicationGateway
metadata:
  name: iag-instance
spec:
  configuration:
    - type: oidc_registration
      discoveryEndpoint: https://ibm-app-gw.verify.ibm.com/oidc/endpoint/default/.well-known/openid-configuration
      postData:
        - name: redirect_uris
          values:
            - https://127.0.0.1:30112/pkmsoidc
      secret: oidc-client
```

This definition will result in the operator:

1. Calling the discoveryEndpoint to retrieve the registration endpoint
2. Possibly making a HTTP POST request to the token endpoint to retrieve an authorization token for the registration. The need for this step is dependent on the type of authorization data that has been provided in the Kubernetes secret. For more details on this see [Secret](#secret).
3. Making a HTTP POST request to the registration endpoint to register a new client, passing the specified POST data.
4. Storing the client ID and secret from the response as data entries in the Kubernetes secret named "oidc-client" using the keys "client\_id" and "client\_secret".
5. Merging a new OIDC identity configuration entry into the IBM Application Gateway configuration.

The resulting IBM Application Gateway configuration YAML will include:

```yaml
identity:
  oidc:
    client_id: secret:oidc-client/client_id
    client_secret: secret:oidc-client/client_secret
    discovery_endpoint: https://ibm-app-gw.verify.ibm.com/oidc/endpoint/default/.well-known/openid-configuration
```

##### Discovery Endpoint

As stated above, the discovery endpoint is a required property when configuring dynamic client registration. The entry will be used for 2 purposes:

1. It will be added to the resulting OIDC provider configuration as the "discovery\_endpoint".
2. It will be used by the operator to retrieve the necessary registration endpoint and possibly the token endpoint if required.

As such the OIDC provider must support discovery and dynamic client registration. The response from the GET call to the discovery endpoint must include:

* registration\_endpoint
* token\_endpoint (if an authorization token is required for client registration)

##### POST Data

The request to an OIDC provider to dynamically register a new client will require certain properties to be provided in the form of POST data. The properties are defined as part of the [OpenID Connect Dynamic Client Registration specification](https://openid.net/specs/openid-connect-registration-1_0.html). Each individual OIDC provider implementation may also include their own list of additional properties that may be set.

IBM Security Verify properties are defined in the [IBM Knowledge Center](https://www.ibm.com/support/knowledgecenter/en/SSCT62/com.ibm.iamservice.doc/tasks/t_dynamic_kc.html).

When using the `IBMApplicationGateway` custom resource the POST data properties may be added in one of 2 ways:

1. As a single string value using the "value" YAML key; or
2. As an array of string values using the "values" YAML key

If an entry contains both "value" and "values" the single entry "value" will take precedence and the "values" entry will be ignored.

The following YAML extract illustrates how to set the POST data:

```yaml
postData:
  - name: redirect_uris
    values:
      - https://127.0.0.1:30112/pkmsoidc
      - https://127.0.0.1:30113/pkmsoidc
  - name: client_name
    value: IAGInstance
  - name: enforce_pkce
    value: "false"
  - name: all_users_entitled
    value: "true"
  - name: consent_action
    value: never_prompt
``` 

This definition will result in the following POST data being sent to the client registration endpoint:

```
{
	"redirect_uris":["https://127.0.0.1:30112/pkmsoidc", "https://127.0.0.1:30113/pkmsoidc"],
	"client_name": "IAGInstance",
	"enforce_pkce": "false",
	"all_users_entitled": "true",
	"consent_action": "never_prompt"
}
```

> Note: While the primary use of the POST data entry is to provide data for the registration request the `scopes` entry (if specified) will also be used in the request to the token endpoint if an authorization token needs to be retrieved.

##### Secret

The OIDC registration definition requires a valid and existing Kubernetes secret. This secret has a dual purpose:

1. The operator will retrieve the authorization data used to authorize the OIDC dynamic client registration.
2. Once the new client has been registered, the new client ID and secret will be stored in the secret and referenced by the IBM Application Gateway configuration.

The authorization data can be specified in 4 different ways. The following table shows the different methods. The precedence shows the order in which the operator will attempt to lookup the secret data. If the data for a particular method is found, that method will be used for authorization.  

Precedence | Secret Data      | Encoding | Description | When to use |
---------- | --------------   | -------- | ----------- | ----------- |
1 | baUsername, baPassword | base64 | Client registration will be attempted with a BA authorization header. | The OIDC provider supports basic authentication to authorize the client registration. |
2 | initialAccessToken | base64 | Client registration will be attempted with a bearer token authorization header. | A token exists that will not expire or will be managed manually. |
3 | tokenRetrievalClientId, tokenRetrievalClientSecret | base64 | The ID and secret will be used to firstly authorize a token retrieve request to the OIDC provider. This token will subsequently be used to authorize the client registration. | The authorization token cannot be stored long term but an intermediate client ID and secret exists that can be used to retrieve a token on demand. |
4 | None | None | Client registration will be attempted without an authorization header. | The OIDC provider does not require authorization for client registration. |

For example, the following YAML shows a Kubernetes secret that contains the tokenRetrievalClientId, tokenRetrievalClientSecret and the created client\_id and client\_secret that were returned by the OIDC provider.

```yaml
apiVersion: v1
kind: Secret
type: Opaque
data:
  tokenRetrievalClientId: YzEwZTVlPKLDTAxNy00OWI4LWI3ZTItNjAwYWY5MzEwZTA5Cg==
  tokenRetrievalClientSecret: dtHdEsfk4Mgo=
  client_id: sfUfJfsfSfgdfdf1j5kk45=
  client_secret: klj345a9HeLH234JKjjk
```

If the tokenRetrievalClientId and tokenRetrievalClientSecret are specified the OIDC provider discovery request must return the token endpoint. In this case the client registration will result in three HTTP calls:

1. GET discovery\_endpoint (No authorization).
2. POST token\_endpoint (BA using tokenRetrievalClientId and tokenRetrievalClientSecret).
3. POST registration\_endpoint (Bearer token using the response from step 2).

#### OIDC Client Lifecycle Management

The IBM Application Gateway operator will call the OIDC provider to register a new client if an OIDC registration definition is provided. Once the client has been registered the ID and secret are stored in a Kubernetes secret. At this point the operator and IBM Application Gateway instance will continue to use the registered client even if it expires or is deleted from the OIDC provider. Note that the operator will not register a new client if the specified secret already contains a client\_id and client\_secret field.

The administrator is responsible for any further lifecycle management of the client.

If a new client registration is required the administrator should:

1. Update the secret to remove the client\_id and client\_secret.
2. Trigger an update of the IBM Application Gateway definition.

The operator will then be invoked and should register a new client with the OIDC OP and update the IBM Application Gateway instance identity provider.

#### Trust OIDC OP Certificate

The IBM Application Gateway operator will load its own CA certificates by default in order to validate trust with the OIDC provider. There may be scenarios whereby the OIDC provider certificate cannot be trusted by default. In this case there are two methods that can alleviate the problem.

1. For a non production scenario where the trust validation is not required a flag named insecureTLS can be set in the Kubernetes secret data that will disable trust validation. This is the same secret that has been specified in the OIDC registration definition. Setting the flag to true will disable trust validation. 

> Note: As the flag is being set in a Kubernetes secret the value must be base64 encoded. First use a base64 encoding tool to get the value to set. For example on linux:

```shell
$ echo -n "true" | base64
dHJ1ZQ==
```

```yaml
apiVersion: v1
kind: Secret
type: Opaque
data:
  insecureTLS: dHJ1ZQ==
```

2. The OIDC provider certificate can be added to the Kubernetes operator service account data and it will then be added to the trusted certificates when the call to the OIDC provider is made. The certificate should be extracted from the OIDC provider and added to the operator service account token secret as a new entry "service-ca.crt".  For example. If the operator is running with the service account "ibm-application-gateway-operator" there will be a service account token secret named "ibm-application-gateway-operator-token-\<random\>" that exists in Kubernetes. Add the base 64 encoded certificate to this secret.

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/service-account-token
data:
  service-ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk.....
```

### Custom Resource Model

An IBM Application Gateway instance deployed on Kubernetes is a complex deployment. In particular the configuration can be defined externally in one or more locations and changes to this configuration may require all instances to be reloaded for the changes to take effect. The existing Kubernetes deployment controller does not have any knowledge of how an IBM Application Gateway instance should behave when the configuration changes. 

In the custom resource model the operator will monitor for the updates to resources of the kind `IBMApplicationGateway` and use these custom resources to manage the lifecycle of the IBM Application Gateway instances.

#### Custom Object

A custom object, of the kind `IBMApplicationGateway`, is used by the operator to manage the lifecycle of the IBM Application Gateway instances.  Each of these custom objects, once created in Kubernetes, will result in an IBM Application Gateway deployment being created by the operator.

> Warning: Prior to creating the custom object in Kubernetes ensure that any referenced secrets, configmaps or external web sources exist.

The following YAML file (iag-instance.yaml) contains an example custom resource:

```yaml
apiVersion: ibm.com/v1
kind: IBMApplicationGateway
metadata:
  name: iag-instance
spec:
  replicas: 3
  deployment:
    serviceAccountName: ibm-application-gateway
    image: icr.io/ibmappgateway/ibm-application-gateway:22.07.0 
    imagePullPolicy: IfNotPresent 
  configuration:
    - type: configmap
      name: iag-config
      dataKey: config
    - type: literal 
      value: |
        version: "22.07"
        resource_servers:
          - path: /app
            connection_type: tcp
            servers:
              - host: 10.0.0.179
                port: 8079
```

The YAML file must include:

1. The kind set as `IBMApplicationGateway`. This will result in the IBM Application Gateway operator being notified of any changes to this object.
2. The image. This is the location and version of the IBM Application Gateway image used to create new pods. If the image location requires authorization details for access these can be added as a Kubernetes secret and the secret reference added here using the imagePullSecrets field. 
3. The configuration for the IBM Application Gateway instances, which may be defined in one or more different locations. 

The following command can be used to create the custom resource from this file:

```shell
kubectl apply -f iag-instance.yaml
```

> Ensure that the ibm-application-gateway service account has been created before creating the custom object above.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ibm-application-gateway
```

```shell
kubectl apply -f iag_service_account.yaml
```

The IBM Application Gateway operator provides a mechanism for the configuration to be defined in more than one location. The configuration from the multiple locations will be merged into a single config map that will be used by the running IBM Application Gateway instance.

The configuration can be defined with one or more of:

* A literal definition in the custom object. Use the YAML configuration type entry "literal".
* A config map reference in the custom object. Use the YAML configuration type entry "configmap".
* A RESTful web location reference in the custom object. Use the YAML configuration type entry "web".
* An OIDC dynamic client registration definition in the custom object. Use the YAML configuration type entry "oidc_registration". 

This provides the ability to split the ownership of the different parts of the configuration into the various application roles. ie:

* developers
* security
* auditors
* devops
* etc

##### Literal Source

A part or all of the IBM Application Gateway configuration can be defined in the custom object YAML. There can be multiple literal definitions in the same YAML configuration and each will be merged separately. 

```yaml
apiVersion: ibm.com/v1
kind: IBMApplicationGateway
metadata:
  name: iag-instance
spec:
  configuration:
    - type: literal
      value: |
        version: "22.07"

        resource_servers:
          - path: /app
            connection_type: tcp
            servers:
              - host: 10.0.0.179
                port: 8079
```

> Changes made to the literal configuration will result in the operator being notified and the running IBM Application Gateway instance will be updated automatically.

##### Config Map Source

If a part or all of the IBM Application Gateway configuration is to be stored in an existing config map the config map must exist prior to creating the custom object. If it does not exist the creation of the IBM Application Gateway instance will fail.

```yaml
apiVersion: ibm.com/v1
kind: IBMApplicationGateway
metadata:
  name: iag-instance
spec:
  configuration:
    - type: configmap
      name: iag-config
      dataKey: config
```

This custom resource will require an existing config map, named "iag-config", and in that config map the data key for the IBM Application Gateway configuration must be "config".

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: iag-config
data:
  config: |
    ...
    ...
```

> Changes made to a referenced config map will result in the operator being notified and any running IBM Application Gateway instances that reference the changed config map will be updated automatically.

##### Web Source

This source type is used if a part or all of the IBM Application Gateway configuration is to be stored in an external web location. The web location must exist and be accessible prior to creating the custom object. If not, the creation of the IBM Application Gateway instance will fail.

For a simplistic web location, that does not require any authorization or other HTTP headers in the GET request, only the URL is required:

```yaml
apiVersion: ibm.com/v1
kind: IBMApplicationGateway
metadata:
  name: iag-instance
spec:
  configuration:
    - type: web
      url: https://raw.github.com/iag_config/config.yaml
```          

For a more advanced web location, that does require additional information in the form of HTTP headers, the headers must also be defined:

```yaml
apiVersion: ibm.com/v1
kind: IBMApplicationGateway
metadata:
  name: iag-instance
spec:
  configuration:
    - type: web
      url: https://raw.github.ibm.com/iag/master/config.yaml
      headers:
        - name: Authorization
          type: literal
          value: "Token 2432347234af7897c9877d908098"
        - name: x-ibm-key
          type: secret
          value: my_ibm_secret
          secretKey: secret_value
``` 

The above example will add 2 headers to the HTTP GET request:

1. "Authorization: Token 2432347234af7897c9877d908098". The value for this header is extracted from the literal text in the YAML definition. This must be in clear text and include the full text required for the header value.
2. "x-ibm-key: <the secret>". The value for this header is extracted from a Kubernetes secret named "my\_ibm\_secret". This secret MUST have the base64 encrypted value with the key that has been specified. For example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my_ibm_secret
type: Opaque
data:
  secret_value: sdfjhw343412ehajhakhdq3==
```    

> Changes made to any external web config sources will not result in the operator being notified. 

###### Web Configuration Updates

Changes to either literal or config map configuration sources will result in the IBM Application Gateway operator being notified and the running instances being updated as required. The web source differs in that there is no listener that is notified or checks for changes to the remote configuration. As such if the external web configuration is updated there are 2 manual steps that must be run in Kubernetes for the changes to take effect.

First, perform a manual update of any running deployments that reference the external web config:

```shell
kubectl rollout restart deployment.apps/<iag-instance>
```

Next, optionally update the revision history to add the change cause:

```shell
kubectl edit deployment.apps/<iag-instance>
```

Set the change cause annotation and save:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    kubernetes.io/change-cause: External web config changed.
...
...
```

##### OIDC Registration Configuration Source

A single oidc_registration configuration source may be added to the list of sources defined in the custom resource definition. Defining more than one oidc_registration source will result in an error. The operator requires the following properties to be specified:

* discoveryEndpoint. This is the endpoint that can be used to discover the registration endpoint and token endpoint of the OIDC provider.
* postData. Specifies any POST data that is required to be sent as part of the registration request.
* secret. Specifies a Kubernetes secret that may contain authorization data for the registration request. This is also the location where the resulting client ID and secret are stored upon successful registration.

```yaml
apiVersion: ibm.com/v1
kind: IBMApplicationGateway
metadata:
  name: iag-instance
spec:
  configuration:
    - type: oidc_registration
      discoveryEndpoint: https://ibm-app-gw.verify.ibm.com/oidc/endpoint/default/.well-known/openid-configuration
      postData:
        - name: redirect_uris
          values:
            - https://127.0.0.1:30112/pkmsoidc
      secret: oidc-client
```

The oidc\_registration source will always be handled last by the operator when merging the list of configuration sources. This is to ensure that the newly registered OIDC client is not overwritten by other sources. If other sources do define their own identity sources the following will occur:

1. If the existing identity provider is an OIDC provider, the new client ID and secret along with the discovery endpoint will be merged into the existing OIDC identity configuration.
2. If the existing identity provider is not an OIDC provider it will be removed and the new OIDC identity provider will be used instead.

#### Custom Object changes

The IBM Application Gateway custom objects are constantly being monitored by the operator. Any significant changes will result in the running pods being reloaded with the new configuration. 

Note that in this context a reload of an IBM Application Gateway application means a rolling update of each running pod. The running pods will not be deleted until a new replacement pod with the changes has been created and is available. In this way there will be no disruption to service. If the update fails the existing pods will remain running.

The following changes to custom objects are considered significant, resulting in a pod reload (any other changes will require a manual reload if desired):

1. Replica count
2. Language
3. Service Account
4. Literal configuration definition
5. Changes to a referenced config map
6. Upgrading an IBM Application Gateway instance by changing the image location

##### Changing Replica Count

To change the number of replicas for a running IBM Application Gateway application:

1. Modify the YAML file for the required custom object by setting the replicas entry to the desired number
2. Apply the changes:

```shell
kubectl apply -f iag-instance.yaml
```

At this point the operator should apply the change by creating/removing pods as required.

##### Changing Language

To change the language in which the IBM Application Gateway error messages are displayed:

1. Modify the YAML file for the required custom object by setting the lang entry to one of the following:

Value      | Language 
--------   | --------
pt | Brazilian Portuguese
cs | Czech
zh_CN | Chinese (Simplified)
zh_TW | Chinese (Traditional)
C | English
fr | French
de | German
hu | Hungarian
it | Italian
ja | Japanese
ko | Korean
pl | Polish
es | Spanish
ru | Russian

2. Apply the changes:

```shell
kubectl apply -f iag-instance.yaml
```

At this point the operator should apply the change by reloading each running pod.

##### Changing the Service Account

To change the Kubernetes service account that the IBM Application Gateway application is running as:

1. Ensure that the new service account exists in the Kubernetes environment
2. Modify the YAML file for the required custom object by setting the serviceAccountName entry to the desired value
3. Apply the changes:

```shell
kubectl apply -f iag-instance.yaml
```

At this point the operator should apply the change by reloading each running pod.

##### Changing the Literal Configuration

To change the literal configuration that the IBM Application Gateway application is using:

1. Modify the YAML file for the required custom object by setting the literal configuration entry to the desired value
2. Apply the changes:

```shell
kubectl apply -f iag-instance.yaml
```

At this point the operator should apply the change by reloading each running pod.

Remember that for multiple configuration entries the sources are merged to produce a master configuration. This merging is done in the order that they are defined. Changes to an earlier configuration source may NOT result in an actual change if a later source defines the same entry.

##### Changing a Referenced Config Map

To change the configuration that the IBM Application Gateway application is using from a referenced config map:

1. Modify the YAML file for the config map with the desired changes
2. Apply the changes:

```shell
kubectl apply -f iag-config.yaml
```

At this point the operator should apply the change by reloading each running pod.

Remember that for multiple configuration entries the sources are merged to produce a master configuration. This merging is done in the order that they are defined. Changes to an earlier configuration source may NOT result in an actual change if a later source defines the same entry.

##### Upgrading an IBM Application Gateway instance by changing the image location

To upgrade a running IBM Application Gateway instance, change the image location:

1. Modify the YAML file for the required custom object by setting the image location entry to the desired value
2. Apply the changes:

```shell
kubectl apply -f iag-instance.yaml
```

At this point the operator should apply the change by recreating each running pod with the new image.

#### Revision History

The IBM Application Gateway operator defines any custom resources as Kubernetes deployments. This means that the deployment rollout history is also maintained. When deployment updates are made the operator will tag the replica set with the reason for the change. 

To view the revision history:

```shell
kubectl rollout history deployment.apps/<iag-instance>

deployment.apps/iag-instance
REVISION  CHANGE-CAUSE
1         <none>
2         Configuration change
3         Image changed from "x" to "y"
4         Service account changed from "x" to "y"
5         Language changed from en to fr
```

The following deployment roll back operation is *not* supported for the operator deployment.

```shell
kubectl rollout undo deployment.apps/iag-instance
```

The stored revision history only saves the deployment settings. The operator deployment uses the IBM Application Gateway custom resource settings and one or more config maps. These are not maintained as part of the revision history and as such any roll back will attempt to revert to the previous deployment but the operator will update the deployment based upon the custom object and config maps.

#### Deleting an IBM Application Gateway Custom Resource

If an existing deployment of an IBM Application Gateway custom resource is no longer required the custom resource should be deleted.

```shell
kubectl delete IBMApplicationGateways/<instance_name>
```

This will delete from Kubernetes:

1. The custom resource
2. The running IBM Application Gateway instance deployment
3. The running IBM Application Gateway instance pods
4. The current and saved IBM Application Gateway instance replica sets
5. The master merged IBM Application Gateway config map

#### Split Configuration Example

The following IBM Application Gateway custom object YAML file snippet shows an example of how the configuration can be split between different locations

```yaml
apiVersion: ibm.com/v1
kind: IBMApplicationGateway
metadata:
  name: iag-instance
spec:
  configuration:
    - type: configmap
      name: iag-config
      dataKey: config
    - type: web
      url: https://raw.github.ibm.com/iag/master/config.yaml
      headers:
        - name: Authorization
          type: literal
          value: "Token 2432347234af7897c9877d908098"
    - type: literal
      value: |
        version: "22.07"

        resource_servers:
          - path: /app
            connection_type: tcp
            servers:
              - host: 10.0.0.179
                port: 8079
```

The config map named "iag-config" is defined as

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: iag-config
data:
  config: |
    version: "22.07"

    server:
      local_applications:
        cred_viewer:
          path_segment: creds
          enable_html: true

    resource_servers:
      - path: /app2
        connection_type: tcp
        servers:
          - host: 10.0.0.111
            port: 80
```

The web config (https://raw.github.ibm.com/iag/master/config.yaml) is defined as

```yaml
version: "22.07"

resource_servers:
  - path: /app3
    connection_type: tcp
    servers:
      - host: 10.0.0.111
        port: 80
```

The different sources of configuration are processed in the order they are defined in the custom object YAML file, and merged with the previous defined configuration to create a final master config map that is used by the IBM Application Gateway instance.

```yaml
apiVersion: v1
data:
  config.yaml: |
    version: "22.07"

      resource_servers:
        - path: /app
          connection_type: tcp
          servers:
            - host: 10.0.0.179
              port: 8079
        - path: /app2
          connection_type: tcp
          servers:
            - host: 10.0.0.111
             port: 80
        - path: /app3
          connection_type: tcp
          servers:
            - host: 10.0.0.111
              port: 80

      server:
        local_applications:
          cred_viewer:
            path_segment: creds
            enable_html: true
```

The name of the new master config map is derived from the IBM Application Gateway instance name. For example if the instance name is iag-instance, a new master config map will be created with the name iag-instance-config-iag-internal-generated9rdnh.

Note that the merging is performed in order. If duplicate entries are defined in more than one location, the final location will contain the value that will be used in the master configuration. The only exception to this is when the entries are array entries. In this case the final merged config will contain all of the configured entries. For an example of this refer to the resource_server entries in the previous example.

> Warning: If multiple sources each define the same array entries, this may result in the merged config being invalid and the IBM Application Gateway instances failing to start.

#### Hello World Example

A simple hello world example which shows how to protect an application using a custom resource can be found at: [docs/custom-resource-example.md](docs/custom-resource-example.md).

An additional hello world example, which shows how to dynamically register the client with the OIDC provider, can be found at: [docs/dynamic-client-example.md](docs/dynamic-client-example.md).

### Sidecar Model

A Kubernetes application may consist of a number of different components. These may include:

* Main application
* Security
* Logging
* Monitoring
* etc

Separating these components into their own container provides isolation and encapsulation. Each container provides a single capability without needing to know the intricate details of the other components.

In Kubernetes this pattern is known as a sidecar. Each sidecar container is attached to the main application and provides a specific function. 

The IBM Application Gateway supports the sidecar pattern such that it can be configured to run alongside the main application providing authorization capabilities without the application needing to know the details.

#### Annotations

The IBM Application Gateway operator will be called by Kubernetes for each deployment management request. This means that there needs to be a method by which the IBM Application Gateway operator can determine whether or not to perform any mutation on the request. The IBM Application Gateway operator will check the deployment annotations to decide whether or not to handle container modifications.

If any of the annotation keys has a prefix of "ibm-application-gateway.security.ibm.com/" the IBM Application Gateway operator will mutate the request. The annotation keys and values are used to determine how the IBM Application Gateway sidecar container will be configured.

The supported annotations are mapped into the following categories:

1. Deployment
2. Service
3. Configuration
4. Environment

##### Deployment annotations

Deployment annotations define how the IBM Application Gateway sidecar container deployment will be generated. The supported deployment keys are:

| Name | Description |
|----------|---------|
|ibm-application-gateway.security.ibm.com/deployment.image | The name, tag and location of the IBM Application Gateway docker image. This is a required value and if not specified, or the value is incorrect, the request will fail. |
|ibm-application-gateway.security.ibm.com/deployment.imagePullPolicy | The policy used to decide when to pull the IBM Application Gateway docker image from a remote server. If not specified the value will be set to ifNotPresent. |

> Note: If an imagePullSecret is required to pull the image it must be defined in the application deployment YAML.

Example:

```yaml
ibm-application-gateway.security.ibm.com/deployment.image: icr.io/ibmappgateway/ibm-application-gateway:22.07.0
ibm-application-gateway.security.ibm.com/deployment.imagePullPolicy: IfNotPresent
```

##### Service annotations

The IBM Application Gateway admission controller may add a new IBM Application Gateway sidecar container alongside the application. To be able to access the 8443 port of the sidecar container a new service may be required. If the service annotation is specified, the admission controller will create the new service exposing the port. 

> This new service will be created by the operator. This means that the service will exist even if the Kubernetes deployment operation fails.

The supported service keys are:

| Name | Description |
|----------|---------|
|ibm-application-gateway.security.ibm.com/service.port | The port to expose the internal 8443 port on. If not specified the service will not be created. |

Example:

```yaml
ibm-application-gateway.security.ibm.com/service.port: "30441"
```

This definition will result in a new service being created:

```yaml
apiVersion: v1
kind: Service
metadata:
  creationTimestamp: "2020-06-10T06:05:45Z"
  generateName: appname-ibm-application-gateway-sidecar-svc
  labels:
    app: appname-ibm-application-gateway-sidecar-svc
  name: appname-ibm-application-gateway-sidecar-svcrdfqh
  namespace: default
spec:
  ports:
  - name: appname-ibm-application-gateway-sidecar-svc
    nodePort: 30441
    port: 8443
    protocol: TCP
    targetPort: 8443
  selector:
    app: appname-ibm-application-gateway-sidecar-pod
  type: NodePort  
```

##### Configuration annotations

The IBM Application Gateway sidecar container requires YAML configuration in order for it to run. The configuration can be created in one or more Kubernetes configmaps and/or one or more external web sources. The configuration sources are merged into a master configmap that is made available to the IBM Application Gateway sidecar container.

> The new master configmap will be created by the admission controller. This means that the configmap will exist even if the Kubernetes deployment operation fails.

The supported configuration keys include:

| Name | Description |
|----------|---------|
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.type | The type of the configuration source. The id must be unique for each separate source. The supported values are "configmap", "web" or "oidc\_registration". Note that there can only be a single oidc\_registration entry. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.order | The order in which to merge the configuration source into the master configmap. Later merges will overwrite any earlier values apart from array entries where the master configmap will contain all specified array entries from all sources. Note that the oidc\_registration entry will always be merged last. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.name | The name of the config map that contains the IBM Application Gateway configuration. Required for configmap type. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.dataKey | The config map YAML entry that contains the IBM Application Gateway configuration. Required for configmap type. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.url | The URL location of the remote IBM Application Gateway configuration. Required for web type. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.header.\<hdrid\>.type | The type of header value to add to the request. A literal type will add the value directly to the new header. A secret type will lookup a Kubernetes secret to retrieve the value. The hdrid must be unique for each header. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.header.\<hdrid\>.name | The name of the header that will be added to the HTTP request. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.header.\<hdrid\>.value | The value of the header that will be added to the HTTP request. If the type is set as secret this will be the name of the Kubernetes secret. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.header.\<hdrid\>.secretKey | The key name to retrieve the header value from the specified Kubernetes secret. Required if the type is set as secret. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.discoveryEndpoint | The endpoint that can be used to discover the registration endpoint and token endpoint of the OIDC provider. Required for oidc\_registration type. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.secret | Specifies a Kubernetes secret that may contain authorization data for the registration request. This is also the location where the resulting client ID and secret are stored upon successful registration. Required for oidc\_registration type. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.postData.\<pdid\>.name | The name of a POST data entry that will be added to the registration request as POST data. Only valid for oidc\_registration type. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.postData.\<pdid\>.value | A single value of the POST data entry that will be added to the registration request as POST data. Only valid for oidc\_registration type. |
|ibm-application-gateway.security.ibm.com/configuration.\<id\>.postData.\<pdid\>.values.\<valueid\> | A value that will be added to an array of values for the POST data entry, used in the registration request. This will be ignored if a single "value" has also been set. Only valid for oidc\_registration type. |

Example:

```yaml
ibm-application-gateway.security.ibm.com/configuration.test.type: configmap
ibm-application-gateway.security.ibm.com/configuration.test.name: test-config
ibm-application-gateway.security.ibm.com/configuration.test.dataKey: config
ibm-application-gateway.security.ibm.com/configuration.test.order: "1"
ibm-application-gateway.security.ibm.com/configuration.sample.type: "web"
ibm-application-gateway.security.ibm.com/configuration.sample.url: https://raw.github.ibm.com/IAG/iag-config/master/test/sample1.yaml
ibm-application-gateway.security.ibm.com/configuration.sample.order: "2"
ibm-application-gateway.security.ibm.com/configuration.sample.header.authz.type: secret
ibm-application-gateway.security.ibm.com/configuration.sample.header.authz.name: Authorization
ibm-application-gateway.security.ibm.com/configuration.sample.header.authz.value: githubsecret
ibm-application-gateway.security.ibm.com/configuration.sample.header.authz.secretKey: value
ibm-application-gateway.security.ibm.com/configuration.oidc.type: oidc_registration
ibm-application-gateway.security.ibm.com/configuration.oidc.discoveryEndpoint: https://ibm-app-gw.verify.ibm.com/oidc/endpoint/default/.well-known/openid-configuration
ibm-application-gateway.security.ibm.com/configuration.oidc.secret: oidc-client
ibm-application-gateway.security.ibm.com/configuration.oidc.postData.rduri.name: redirect_uris
ibm-application-gateway.security.ibm.com/configuration.oidc.postData.rduri.values.value1: https://127.0.0.1:30112/pkmsoidc
ibm-application-gateway.security.ibm.com/configuration.oidc.postData.cn.name: client_name
ibm-application-gateway.security.ibm.com/configuration.oidc.postData.cn.value: OperatorTest
ibm-application-gateway.security.ibm.com/configuration.oidc.postData.aue.name: all_users_entitled
ibm-application-gateway.security.ibm.com/configuration.oidc.postData.aue.value: "true"
ibm-application-gateway.security.ibm.com/configuration.oidc.postData.ep.name: enforce_pkce
ibm-application-gateway.security.ibm.com/configuration.oidc.postData.ep.value: "false"
ibm-application-gateway.security.ibm.com/configuration.oidc.postData.ca.name: consent_action
ibm-application-gateway.security.ibm.com/configuration.oidc.postData.ca.value: "never_prompt"
ibm-application-gateway.security.ibm.com/configuration.oidc.order: "3"
```

This definition will result in a new master configmap being created:

```yaml
apiVersion: v1
data:
  config.yaml: |
    identity:
      oidc:
        client_id: secret:oidc-client/client_id
        client_secret: secret:oidc-client/client_secret
        discovery_endpoint: https://ibm-app-gw.verify.ibm.com/oidc/endpoint/default/.well-known/openid-configuration
    resource_servers:
    - connection_type: tcp
      path: /sample
      servers:
      - host: 10.0.0.12
        port: 80
    - connection_type: tcp
      path: /eai
      servers:
      - host: 10.0.0.15
        port: 8080
    server:
      local_applications:
        cred_viewer:
          enable_html: false
          path_segment: creds
    version: "22.07"
kind: ConfigMap
metadata:
  creationTimestamp: "2020-06-10T06:05:47Z"
  generateName: appname-ibm-application-gateway-sidecar-configmap
  labels:
    app: appname-ibm-application-gateway-sidecar-pod
  name: appname-ibm-application-gateway-sidecar-configmap9c8dm
  namespace: default
```

The config.yaml data value is a merging of the three defined configuration sources.

##### Environment annotations

The IBM Application Gateway sidecar container may require certain environment variables to be set to operate correctly. Environment annotations allow these environment variable values to be set.

The supported environment keys are:

| Name | Description |
|----------|---------|
|ibm-application-gateway.security.ibm.com/env.\<name\> | This will result in a new environment variable being set where \<name\> specifies the name of the variable (case sensitive) and the specified value. |

Example:

```yaml
ibm-application-gateway.security.ibm.com/env.LANG: fr
```

This definition will result in a new environment variable being set: "LANG=fr".

#### Supported RESTful operations

##### Create

When a deployment is first created the IBM Application Gateway operator will be called with a RESTful create operation. The operator will perform the following tasks:

1. Check to see if the deployment annotations contain any keys with the prefix "ibm-application-gateway.security.ibm.com/". If not, the operator will make no changes and return.
2. Run some validation on the annotations to try and ensure that no simple failures will occur which might leave the environment in an intermediate state.
3. Check to see if the annotations contain a service port. If so, a new service will be created.
4. Read and merge the configuration sources to create a new master configmap.
5. Return a new IBM Application Gateway container patch as the response. This will result in the IBM Application Gateway sidecar being created alongside the application.

##### Update

When a deployment is modified the IBM Application Gateway operator will be called with a RESTful update operation. The operator will perform the following tasks:

1. Check to see if the deployment annotations contain any keys with the prefix "ibm-application-gateway.security.ibm.com/". If not, the operator will make no changes and return.
2. Run some validation on the annotations to try and ensure that no simple failures will occur which might leave the environment in an intermediate state.
3. Check to see if the annotations contain a service port. If so, the service will be updated if the port has changed.
4. Read and merge the configuration sources to create a new master configmap.
5. Return a new IBM Application Gateway container patch as the response. This will result in the IBM Application Gateway sidecar being updated alongside the application.

##### Delete 

When a deployment is deleted the IBM Application Gateway operator will be called with a RESTful delete operation. The operator will perform the following tasks:

1. Check to see if the deployment annotations contain any keys with the prefix "ibm-application-gateway.security.ibm.com/". If not, the operator will make no changes and return.
2. Delete the service if it exists.
3. Delete the master configmap.

#### Example Deployment

Assuming that the IBM Application Gateway operator has been setup and deployed, the following deployment will result in an IBM Application Gateway sidecar container being created alongside the application:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: testapp
  labels:
    app: testapp
    name: testapp
  annotations:
    ibm-application-gateway.security.ibm.com/env.LANG: en
    ibm-application-gateway.security.ibm.com/configuration.test.type: configmap
    ibm-application-gateway.security.ibm.com/configuration.test.name: test-config
    ibm-application-gateway.security.ibm.com/configuration.test.dataKey: config
    ibm-application-gateway.security.ibm.com/configuration.test.order: "1"
    ibm-application-gateway.security.ibm.com/configuration.sample.type: "web"
    ibm-application-gateway.security.ibm.com/configuration.sample.url: https://raw.github.ibm.com/IAG/iag-config/master/test/sample1.yaml
    ibm-application-gateway.security.ibm.com/configuration.sample.order: "2"
    ibm-application-gateway.security.ibm.com/configuration.sample.header.authz.type: secret
    ibm-application-gateway.security.ibm.com/configuration.sample.header.authz.name: Authorization
    ibm-application-gateway.security.ibm.com/configuration.sample.header.authz.value: githubsecret
    ibm-application-gateway.security.ibm.com/configuration.sample.header.authz.secretKey: value
    ibm-application-gateway.security.ibm.com/deployment.image: icr.io/ibmappgateway/ibm-application-gateway:22.07.0
    ibm-application-gateway.security.ibm.com/deployment.imagePullPolicy: IfNotPresent
    ibm-application-gateway.security.ibm.com/service.port: "30441"
spec:
  selector:
    matchLabels:
      app: testapp
      name: testapp
  replicas: 1
  template:
    metadata:
      labels:
        app: testapp
        name: testapp
    spec:
      imagePullSecrets:
        - name: regcred
      containers:
      - name: testapp
        image: alpine
        command:
          - /bin/sh
          - "-c"
          - "sleep 70m"
        imagePullPolicy: IfNotPresent
``` 

``` shell
kubectl apply -f deployment.yaml
```

```shell
$ kubectl get pods
NAME                                                READY   STATUS             RESTARTS   AGE
testapp-645db9c874-bmq4k                            2/2     Running            1          1d
```

> The testapp pod has 2 containers that are both ready.

```shell
$ kubectl get services
NAME                                             TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)             AGE
testapp-ibm-application-gateway-sidecar-svcrdfqh NodePort    10.109.167.144   <none>        8443:30441/TCP      1d
```

```shell
$ kubectl get configmaps
NAME                                                   DATA   AGE
test-config                                            1      6d
testapp-ibm-application-gateway-sidecar-configmap9c8dm 1      1d
```

#### Hello World Example

A simple hello world example which shows how to protect an application using the sidecar model can be found at: [docs/sidecar-example.md](docs/sidecar-example.md).

## Troubleshooting

There are various ways to try and locate any issues with the IBM Application Gateway operator, or the deployment instances that are created for custom resources.

1. Operator logs

```shell
kubectl logs --follow pod/ibm-application-gateway-operator-7ff6bcffb8-m25g5
```

Look for any errors or log entries that define what has happened.

2. IBM Application Gateway instance logs

```shell
kubectl logs --follow pod/iag-instance3-75bf746b7c-kmzfx
```

Look for any errors or log entries that define what has happened. 

3. Check the operator deployment

```shell
kubectl get deployment.apps/ibm-application-gateway-operator -o yaml
```

The status section may contain event data showing errors.

4. Check the operator pod

```shell
kubectl get pod/ibm-application-gateway-operator-5cd8589ff9-h9jrc -o yaml
```

The status section may contain event data showing errors.

5. Check the custom object deployment

```shell
kubectl get deployment.apps/iag-instance -o yaml
```

The status section may contain event data showing errors.

6. Check the custom object pod

```shell
kubectl get pod/iag-instance3-75bf746b7c-kmzfx -o yaml
```

The status section may contain event data showing errors.

7. Check the master merged config map

```shell
kubectl get configmaps/iag-instance-config-iag-internal-generated -o yaml
```

Check that the generated configuration is as expected.




