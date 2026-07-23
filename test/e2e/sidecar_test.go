/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package e2e

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// sidecarContainerNameFor returns the sidecar container name for a given
// Deployment name, mirroring getAppName() in ibmapplicationgateway_webhook.go.
func sidecarContainerNameFor(deploymentName string) string {
	return strings.ToLower(deploymentName + "-ibm-application-gateway-sidecar-pod")
}

// newSidecarDeployment builds a Deployment object annotated for IAG sidecar
// injection pointing at the named source ConfigMap on the given NodePort.
func newSidecarDeployment(name, namespace, sourceCMName string, nodePort int) *appsv1.Deployment {
	labels := map[string]string{"app": name}
	replicas := int32(1)
	annots := map[string]string{
		"ibm-application-gateway.security.ibm.com/deployment.image":         iagImage,
		"ibm-application-gateway.security.ibm.com/configuration.0.type":     "configmap",
		"ibm-application-gateway.security.ibm.com/configuration.0.name":     sourceCMName,
		"ibm-application-gateway.security.ibm.com/configuration.0.dataKey":  "config.yaml",
		"ibm-application-gateway.security.ibm.com/configuration.0.order":    "1",
	}
	if nodePort > 0 {
		annots["ibm-application-gateway.security.ibm.com/service.port"] = strconv.Itoa(nodePort)
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annots,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "busybox:latest"},
					},
				},
			},
		},
	}
}

// waitForSidecarContainer polls until the named sidecar container appears in
// the Deployment template.
func waitForSidecarContainer(t *testing.T, deploymentName, containerName string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var d appsv1.Deployment
		if err := c.Get(context.Background(),
			client.ObjectKey{Name: deploymentName, Namespace: testNamespace}, &d); err != nil {
			return false
		}
		for _, ctr := range d.Spec.Template.Spec.Containers {
			if ctr.Name == containerName {
				return true
			}
		}
		return false
	}, shortTimeout, pollInterval,
		"sidecar container %q was not injected into Deployment %q", containerName, deploymentName)
}

// cleanupSidecarResources deletes the sidecar ConfigMap and Service by label.
// Called from t.Cleanup; errors are intentionally ignored.
func cleanupSidecarResources(namespace, sidecarContainerName, deploymentName string) {
	var cmList corev1.ConfigMapList
	if err := c.List(context.Background(), &cmList,
		client.InNamespace(namespace),
		client.MatchingLabels{"app": sidecarContainerName},
	); err == nil {
		for i := range cmList.Items {
			_ = c.Delete(context.Background(), &cmList.Items[i])
		}
	}
	var svcList corev1.ServiceList
	if err := c.List(context.Background(), &svcList,
		client.InNamespace(namespace),
		client.MatchingLabels{"app": deploymentName},
	); err == nil {
		for i := range svcList.Items {
			_ = c.Delete(context.Background(), &svcList.Items[i])
		}
	}
}

// TestSidecarInjection deploys a plain Deployment carrying the IAG sidecar
// annotations and asserts that the webhook:
//   - injects the sidecar container named <deployment>-ibm-application-gateway-sidecar-pod
//   - creates a ConfigMap with label app=<sidecarContainerName>
//   - creates a NodePort Service with label app=<deploymentName> and nodePort 30443
func TestSidecarInjection(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	deploymentName := "iag-sidecar-test"
	sidecarName := sidecarContainerNameFor(deploymentName)

	sourceCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "iag-sidecar-source-cm", Namespace: testNamespace},
		Data:       map[string]string{"config.yaml": "version: \"26.7\"\n"},
	}
	require.NoError(t, c.Create(context.Background(), sourceCM))
	t.Cleanup(func() { _ = c.Delete(context.Background(), sourceCM) })

	dep := newSidecarDeployment(deploymentName, testNamespace, "iag-sidecar-source-cm", 30443)
	require.NoError(t, c.Create(context.Background(), dep))
	t.Cleanup(func() { _ = c.Delete(context.Background(), dep) })
	t.Cleanup(func() { cleanupSidecarResources(testNamespace, sidecarName, deploymentName) })

	// Sidecar container must be injected.
	waitForSidecarContainer(t, deploymentName, sidecarName)

	var injected appsv1.Deployment
	require.NoError(t, c.Get(context.Background(),
		client.ObjectKey{Name: deploymentName, Namespace: testNamespace}, &injected))
	var found bool
	for _, ctr := range injected.Spec.Template.Spec.Containers {
		if ctr.Name == sidecarName {
			found = true
		}
	}
	assert.True(t, found, "container %q not found in Deployment %q", sidecarName, deploymentName)

	// ConfigMap with label app=<sidecarName> must exist.
	require.Eventually(t, func() bool {
		var cmList corev1.ConfigMapList
		return c.List(context.Background(), &cmList,
			client.InNamespace(testNamespace),
			client.MatchingLabels{"app": sidecarName},
		) == nil && len(cmList.Items) > 0
	}, shortTimeout, pollInterval,
		"sidecar ConfigMap with label app=%q was not created", sidecarName)

	// NodePort Service with label app=<deploymentName> and nodePort 30443 must exist.
	require.Eventually(t, func() bool {
		var svcList corev1.ServiceList
		if err := c.List(context.Background(), &svcList,
			client.InNamespace(testNamespace),
			client.MatchingLabels{"app": deploymentName},
		); err != nil || len(svcList.Items) == 0 {
			return false
		}
		for _, svc := range svcList.Items {
			for _, p := range svc.Spec.Ports {
				if p.NodePort == 30443 {
					return true
				}
			}
		}
		return false
	}, shortTimeout, pollInterval,
		"NodePort Service with label app=%q and nodePort 30443 was not found", deploymentName)
}

// TestSidecarDeletionCleansUpResources deletes the annotated Deployment and
// asserts that the webhook removes the sidecar ConfigMap and Service.
// This covers the README "Delete" operation (p.1190).
func TestSidecarDeletionCleansUpResources(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	deploymentName := "iag-sidecar-delete-test"
	sidecarName := sidecarContainerNameFor(deploymentName)

	sourceCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "iag-sidecar-del-source-cm", Namespace: testNamespace},
		Data:       map[string]string{"config.yaml": "version: \"26.7\"\n"},
	}
	require.NoError(t, c.Create(context.Background(), sourceCM))
	t.Cleanup(func() { _ = c.Delete(context.Background(), sourceCM) })

	dep := newSidecarDeployment(deploymentName, testNamespace, "iag-sidecar-del-source-cm", 30443)
	require.NoError(t, c.Create(context.Background(), dep))
	// No Cleanup for dep itself — deletion is the action under test.

	// Wait for the sidecar resources to be created before deleting the Deployment.
	waitForSidecarContainer(t, deploymentName, sidecarName)
	require.Eventually(t, func() bool {
		var cmList corev1.ConfigMapList
		return c.List(context.Background(), &cmList,
			client.InNamespace(testNamespace),
			client.MatchingLabels{"app": sidecarName},
		) == nil && len(cmList.Items) > 0
	}, shortTimeout, pollInterval,
		"sidecar ConfigMap should exist before deletion test")

	// Delete the Deployment — this triggers the webhook delete handler.
	require.NoError(t, c.Delete(context.Background(), dep))

	// The sidecar ConfigMap must disappear.
	require.Eventually(t, func() bool {
		var cmList corev1.ConfigMapList
		if err := c.List(context.Background(), &cmList,
			client.InNamespace(testNamespace),
			client.MatchingLabels{"app": sidecarName},
		); err != nil {
			return false
		}
		return len(cmList.Items) == 0
	}, shortTimeout, pollInterval,
		"sidecar ConfigMap with label app=%q should be deleted after Deployment deletion", sidecarName)

	// The Service must also disappear.
	require.Eventually(t, func() bool {
		var svcList corev1.ServiceList
		if err := c.List(context.Background(), &svcList,
			client.InNamespace(testNamespace),
			client.MatchingLabels{"app": deploymentName},
		); err != nil {
			return false
		}
		return len(svcList.Items) == 0
	}, shortTimeout, pollInterval,
		"sidecar Service with label app=%q should be deleted after Deployment deletion", deploymentName)

	// The Deployment itself must also be gone.
	require.Eventually(t, func() bool {
		var d appsv1.Deployment
		err := c.Get(context.Background(),
			client.ObjectKey{Name: deploymentName, Namespace: testNamespace}, &d)
		return errors.IsNotFound(err)
	}, shortTimeout, pollInterval,
		"Deployment %q should be gone", deploymentName)
}
