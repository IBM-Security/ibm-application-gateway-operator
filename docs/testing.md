# Building
A docker image has been created to help with the build process (although the GitHub action created for the project to perform the official build doesn't actually use this docker image).  Further information on how to build the operator with the image is available in the build image [README](../build.image/README.md).

# Testing

The easiest way to test the image is to use the operator SDK.  The operator framework supports a private image repository for the bundle image, but not for the main operator image.  As a result of this, when testing the operator, at least the main operator image must be pushed to a public image repository (which can include the repository which is local to the OpenShift environment).

The operator can then be installed using the following operator-sdk command (after having performed an 'oc' login):

```shell
operator-sdk run bundle ${IMAGE_TAG_BASE}-bundle:${VERSION}
```

If you want to make the operator available to the OpenShift operator catalog you need to execute the following commands (taken from: [https://docs.openshift.com/container-platform/4.6/operators/admin/olm-managing-custom-catalogs.html](https://docs.openshift.com/container-platform/4.6/operators/admin/olm-managing-custom-catalogs.html)):

```shell
# Build and push a catalog which contains the operator
opm index add -u docker --bundles ${IMAGE_TAG_BASE}-bundle:${VERSION} \
     --from-index quay.io/operatorhubio/catalog:latest \
     --tag ${IMAGE_TAG_BASE}-catalog:${VERSION}
docker push ${IMAGE_TAG_BASE}-catalog:${VERSION}

# Add the catalog to the OpenShift environment.
oc create -f ${CATALOG_SOURCE_YAML}

# Validate that the catalog has been added
oc get pods -n openshift-marketplace
oc get catalogsource -n openshift-marketplace
oc get packagemanifest -n openshift-marketplace
```

The catalog source yaml looks like the following:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ibm-application-gateway-catalog
  namespace: openshift-marketplace
spec:
  displayName: IBM Application Gateway
  publisher: IBM
  sourceType: grpc
  image: ${IMAGE_TAG_BASE}-catalog:${VERSION}
  updateStrategy:
    registryPoll:
      interval: 10m
```
