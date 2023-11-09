#!/bin/sh

##############################################################################
# Copyright contributors to the IBM Application Gateway Operator project
##############################################################################

set -e

# We have an issue where the build sometimes fails due to network issues.  The
# simple fix for this is to retry the failed commands.  This function will
# retry the specified command until it succeeds, or exceeds 5 retry attempts.
retry_command()
{
    set +e

    failed=true
    attempt=1

    while [ $failed = "true" ] ; do
        $*

        if [ $? -eq 0 ] ; then
            failed=false
        elif [ $attempt -lt 5 ] ; then
            echo "Error> command failed (trying again)....."
            sleep 5

            attempt=`expr $attempt + 1`
        else
            echo "Error> command failed......"
            exit 1
        fi
    done

    set -e
}

#
# Enable install of RPMs from the CentOS-8 repository.
#

centos_repo_file="/etc/yum.repos.d/centos.repo"

cat <<EOT >> $centos_repo_file
[CentOS-8_base]
name = CentOS-8 - Base
baseurl = http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os
gpgcheck = 0
enabled = 1

[CentOS-8_appstream]
name = CentOS-8 - AppStream
baseurl = http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os
gpgcheck = 0
enabled = 1
EOT

#
# Install the pre-requisite RedHat RPMs
#

retry_command yum -y install make git rsync zip

#
# The build scripts assume go 1.17, so we want to install the latest 
# 1.17 available.
#

go_toolset_version=$(yum list --showduplicates go-toolset | grep 1.17 | awk '{ print $2 }' | sort -Vr | head -n 1)

retry_command yum -y install go-toolset-${go_toolset_version}

#
# Install kubectl.
#

cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF

retry_command yum install -y kubectl

mkdir -p /root/.kube

#
# Install docker.
#

dnf config-manager --add-repo=https://download.docker.com/linux/centos/docker-ce.repo

retry_command dnf -y install docker-ce 

#
# Install the operator SDK.  This code comes directly from the Operator SDK
# Web site: 
#   https://sdk.operatorframework.io/docs/installation/#install-from-github-release
#

export ARCH=amd64
export OS=$(uname | awk '{print tolower($0)}')
export OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download/v1.15.0

retry_command curl -LO ${OPERATOR_SDK_DL_URL}/operator-sdk_${OS}_${ARCH}

#
# Verify that the operator has been downloaded OK.
#

gpg --keyserver keyserver.ubuntu.com --recv-keys 052996E2A20B5C7E

curl -LO ${OPERATOR_SDK_DL_URL}/checksums.txt
curl -LO ${OPERATOR_SDK_DL_URL}/checksums.txt.asc
gpg -u "Operator SDK (release) <cncf-operator-sdk@cncf.io>" \
    --verify checksums.txt.asc

grep operator-sdk_${OS}_${ARCH} checksums.txt | sha256sum -c -

#
# Install the operator.
#

chmod +x operator-sdk_${OS}_${ARCH} 

mv operator-sdk_${OS}_${ARCH} /usr/local/bin/operator-sdk

#
# Set up the motd file, and ensure that we show this file whenever we
# start a shell.
#

cat > /etc/motd << EOF
This shell can be used to build the IAG Operator docker images.  The
build directory is a local directory within the container, and the source files
are rsynced from the workspace directory (/workspace).  If you want to
manually rsync the source code you can issue the 'resync' command, otherwise
the source code will be automatically sync'ed as a part of the 'make' command.

When running the operator in OpenShift you need to ensure that the published
controller image is publically accessible (i.e. there is no way to supply an
image pull secret).  The easiest way to do this is to create a public
repository in a personal account on Docker Hub.  An alternative is to use the
OpenShift Container Platform registry.  Further information can be found
at: https://docs.openshift.com/container-platform/4.6/registry/securing-exposing-registry.html

In order to be able to publish from the build container you will need to:
   1. Copy the ~/.kube/config file to /root/.kube/config
   2. Perform a docker login to the repository (i.e. 'docker login')

The following make targets can be used:

    help:
        This target will display general help information on all targets
        contained within the Makefile.

    docker-all:
        This target will build the main controller image and push the image
        to the remote docker repository.

    bundle-all:
        This target is used to generate the OLM bundle and push the image to
        the remote docker repository.

    catalog-all:
        This target is used to generate the operator index and catalog, and
        push this to the remote docker repository.

In order to deploy the image, using OLM, to a Kubernetes environment:
    1. operator-sdk olm install
    2. operator-sdk run bundle \${IMAGE_TAG_BASE}-bundle:\${VERSION} 

In order to cleanup the Kubernetes environment:
    1. operator-sdk cleanup ibm-application-gateway-operator
    2. operator-sdk olm uninstall

In order to make the operator catalog available in an OpenShift environment:
    1. make catalog-run
    2. install the operator using the OpenShift console

In order to clean up the operator catalog from an OpenShift environment:
    1. uninstall the operator using the OpenShift console
    2. make catalog-cleanup

EOF

cat >> /etc/bashrc << EOF
help() {
    cat /etc/motd
}

resync() {
    rsync -az /workspace/* /build
}

make() {
    echo "Resyncing the source code...."
    resync

    echo "Performing the make.... "
    /usr/bin/make \$*
}

help

export VERSION=latest
export IMAGE_TAG_BASE=icr.io/ibmappgateway/ibm-application-gateway-operator

EOF

#
# Clean-up the temporary files.
#

rm -f checksums.txt checksums.txt.asc

yum clean all

