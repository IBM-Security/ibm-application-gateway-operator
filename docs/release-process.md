# Introduction

This document contains the release process which should be followed when generating a new release of the IBM Application Gateway operator.

## Version Number

The version number should be of the format: `v<year>.<month>.0`, for example: `v21.10.0`.  There should be no leading zero's in any part of the version number.  For exampe, `v22.02.0` should be `v22.2.0`.


# Generating a GitHub Release

In order to generate a new version of the operator a new GitHub release should be created: [https://github.com/IBM-Security/ibm-application-gateway-operator/releases/new](https://github.com/IBM-Security/ibm-application-gateway-operator/releases/new). 

The fields for the release should be:

|Field|Description
|-----|----------- 
|Tag | The version number, e.g. `v22.2.0`
|Release title | The version number, e.g. `v22.2.0`
|Release description | The resources associated with the \<version\-number> IBM Application Gateway operator release.

After the release has been created the GitHub actions workflow ([https://github.com/IBM-Security/ibm-application-gateway-operator/actions/workflows/build.yaml](https://github.com/IBM-Security/ibm-application-gateway-operator/actions/workflows/build.yaml)) will be executed to generate the build.  This build process will include:

* publishing the generated docker images to DockerHub;
* adding the manifest zip file to the release artifacts in GitHub.

# Publishing to OperatorHub.io

Once a new GitHub release has been generated the updated operator bundle needs to be published to OperatorHub.io.  Information on how to do this can be found at the following URL: [https://k8s-operatorhub.github.io/community-operators/](https://k8s-operatorhub.github.io/community-operators/).

At a high level you need to (taken from: [https://k8s-operatorhub.github.io/community-operators/contributing-via-pr/](https://k8s-operatorhub.github.io/community-operators/contributing-via-pr/)):

1. Test the operator locally.
2. Fork the [GitHub project](https://github.com/k8s-operatorhub/community-operators).
3. Add the operator bundle to the ibm-application-gateway-operator directory.
4. Push a 'signed' commit of the changes.  See [https://k8s-operatorhub.github.io/community-operators/contributing-prerequisites/](https://k8s-operatorhub.github.io/community-operators/contributing-prerequisites/).  The easiest way to sign the commit is to use the `git commit -s -m '<description>'` command to commit the changes.
5. Contribute the changes back to the main GitHub repository (using the 'Contribute' button in the GitHub console).  This will have the effect of creating a new pull request against the main GitHub repository.
6. Monitor the 'checks' against the pull request to ensure that all of the automated test cases pass.
7. Wait for the pull request to be merged.  This will usually happen overnight.

# RedHat Operator Certification

Certification projects are managed through the [RedHat Partner Connect Portal](https://connect.redhat.com/manage/projects).  

At a high level, to certify the operator, you need to:

1. Create a 'certification project' for the operator using the RedHat Partner Connect Portal ([instructions](https://redhat-connect.gitbook.io/partner-guide-for-red-hat-openshift-and-container/certify-your-operator/creating-an-operator-project));
	1. Provide the details of the operator on the 'Settings' tab;
	2. Scan the new image using the 'Scan new image' button on the 'Images' tab;
2. Create a 'certification project' for the operator bundle using the RedHat Partner Connect Portal ([instructions](https://redhat-connect.gitbook.io/partner-guide-for-red-hat-openshift-and-container/certify-your-operator/certify-your-operator-bundle-image));
	1. Provide the details of the operator on the 'Settings' tab;
	2. Test the operator and submit a pull request.  


	> It is important that in the pull request the images contained within the cluster service version file are updated, replacing the tag name with the corresponding sha256 digest.

## Bundle Testing

As a part of the certification process you need to test your bundle.  You can do this locally, or by using the hosted pipeline.  Both mechanisms are not without problems.  

### Local Testing

Instructions on how to run the tests locally are available at: [https://github.com/redhat-openshift-ecosystem/certification-releases/blob/main/4.9/ga/ci-pipeline.md](https://github.com/redhat-openshift-ecosystem/certification-releases/blob/main/4.9/ga/ci-pipeline.md)

I was never able to successfully run the tests in my local OpenShift environment, although after a lot of trial and error I was able to make some limited progress. Some points to note about running the tests locally:

1. You need to create a default storage class (type: no-provisioner);
2. You need to create a new persistent volume using the yaml included below;
3. You need to modify the `templates/workspace-template.yaml` file to reference the new PV: `volumeName: pv0001`

```yaml
kind: PersistentVolume
apiVersion: v1
metadata:
  name: pv0001
spec:
  capacity:
    storage: 50Gi
  nfs:
    server: 10.22.82.15
    path: /data/certify
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Recycle
  storageClassName: manual
  volumeMode: Filesystem
```

### Hosted Pipeline

Instructions on how to run the tests using the hosted pipeline are available at: [https://github.com/redhat-openshift-ecosystem/certification-releases/blob/main/4.9/ga/hosted-pipeline.md](https://github.com/redhat-openshift-ecosystem/certification-releases/blob/main/4.9/ga/hosted-pipeline.md).  


