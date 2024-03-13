# Copyright contributors to the IBM Application Gateway Operator project

# Build the manager binary
FROM golang:1.19 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

# In order to get this operator certified by RedHat it needs to be based on
# RedHat UBI.
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

### Required OpenShift Labels
LABEL name="IBM Application Gatway Operator" \
      vendor="IBM" \
      version="v24.03.0" \
      release="0" \
      summary="This operator adds lifecycle management support for the IBM Application Gateway." \
      description="IBM Application Gateway provides a containerized secure Web Reverse proxy which is designed to sit in front of your application, seamlessly adding authentication and authorization protection to your application.  The IBM Application Gateway operator provides lifecycle management of IBM Application Gateway instances running inside a Kubernetes container."

# Required Licenses
COPY licenses /licenses

ENTRYPOINT ["/manager"]
