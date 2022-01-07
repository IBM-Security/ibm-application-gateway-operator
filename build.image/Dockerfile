##############################################################################
# Copyright contributors to the IBM Application Gateway Operator project
##############################################################################

# Our build image is based on the RedHat Universal Base Image (ubi)
FROM registry.access.redhat.com/ubi8/ubi:latest

# Add in our files.  
ADD root/sbin /sbin

# Execute our script which is used to prepare the image.
RUN /bin/sh /sbin/prepare.sh && rm -f /sbin/prepare.sh

# Our default entry point.
ENTRYPOINT [ "/bin/sh", "/sbin/bootstrap.sh" ]

# The volumes which we need to export.
VOLUME ["/build"]

# Some labels which will be associated with the image.
LABEL maintainer="isamdev@au.ibm.com" \
    vendor="IBM" \
    product="IBM Application Gateway" \
    documentation="http://ibm.biz/ibm-app-gateway" \
    product_information="http://ibm.biz/ibm-app-gateway" \
    copyright="Copyright contributors to the IBM Application Gateway Operator project"

