/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ibmv1 "github.com/ibm-security/ibm-application-gateway-operator/api/v1"
)

const configMapTestCRName = "iag-configmap-test"

// TestConfigMapSourceCreatesDeployment creates an IBMApplicationGateway CR that
// references a ConfigMap as its configuration source and asserts that the
// operator creates a Deployment and master ConfigMap containing the expected data.
func TestConfigMapSourceCreatesDeployment(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	// Source ConfigMap that the CR will reference.
	sourceData := `
version: "26.7"
server:
  local_applications:
    cred_viewer:
      path_segment: "creds"
      enable_html: true
`
	sourceCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "iag-source-cm",
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"config.yaml": sourceData,
		},
	}
	require.NoError(t, c.Create(context.Background(), sourceCM))
	t.Cleanup(func() { _ = c.Delete(context.Background(), sourceCM) })

	// CR referencing the ConfigMap.
	cr := &ibmv1.IBMApplicationGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapTestCRName,
			Namespace: testNamespace,
		},
		Spec: ibmv1.IBMApplicationGatewaySpec{
			Replicas: 1,
			Deployment: ibmv1.IBMApplicationGatewayDeployment{
				ImageLocation:   iagImage,
				ImagePullPolicy: "IfNotPresent",
				Lang:            "C",
			},
			Configuration: []ibmv1.IBMApplicationGatewayConfiguration{
				{
					Type:    "configmap",
					Name:    "iag-source-cm",
					DataKey: "config.yaml",
				},
			},
		},
	}
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, configMapTestCRName, shortTimeout)

	// The master ConfigMap must contain the content from the source ConfigMap.
	waitForMasterConfigMapContent(t, configMapTestCRName, "cred_viewer", shortTimeout)
}

// TestConfigMapUpdateTriggersReconcile updates the source ConfigMap and asserts
// that the master ConfigMap is updated to reflect the new content.
func TestConfigMapUpdateTriggersReconcile(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	// Source ConfigMap v1.
	sourceData := `
version: "26.7"
server:
  local_applications:
    cred_viewer:
      path_segment: "creds-v1"
      enable_html: true
`
	sourceCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "iag-update-source-cm",
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"config.yaml": sourceData,
		},
	}
	require.NoError(t, c.Create(context.Background(), sourceCM))
	t.Cleanup(func() { _ = c.Delete(context.Background(), sourceCM) })

	crName := "iag-update-cm-test"
	cr := &ibmv1.IBMApplicationGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: testNamespace,
		},
		Spec: ibmv1.IBMApplicationGatewaySpec{
			Replicas: 1,
			Deployment: ibmv1.IBMApplicationGatewayDeployment{
				ImageLocation:   iagImage,
				ImagePullPolicy: "IfNotPresent",
				Lang:            "C",
			},
			Configuration: []ibmv1.IBMApplicationGatewayConfiguration{
				{
					Type:    "configmap",
					Name:    "iag-update-source-cm",
					DataKey: "config.yaml",
				},
			},
		},
	}
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	// Wait for initial reconcile.
	waitForDeployment(t, crName, shortTimeout)
	waitForMasterConfigMapContent(t, crName, "creds-v1", shortTimeout)

	// Record the current master ConfigMap ResourceVersion so we can detect the update.
	initial := masterConfigMap(t, crName)
	initialRV := initial.ResourceVersion

	// Patch the source ConfigMap to use a new path_segment.
	updatedData := `
version: "26.7"
server:
  local_applications:
    cred_viewer:
      path_segment: "creds-v2"
      enable_html: true
`
	patch := sourceCM.DeepCopy()
	patch.Data["config.yaml"] = updatedData
	require.NoError(t, c.Update(context.Background(), patch))

	// Wait for the master ConfigMap to be updated (ResourceVersion must change).
	require.Eventually(t, func() bool {
		var cmList corev1.ConfigMapList
		if err := c.List(context.Background(), &cmList,
			client.InNamespace(testNamespace),
			client.MatchingLabels{"app": crName},
		); err != nil || len(cmList.Items) == 0 {
			return false
		}
		return cmList.Items[0].ResourceVersion != initialRV
	}, shortTimeout, pollInterval,
		"master ConfigMap for %q was not updated after source ConfigMap change", crName)

	// Assert new content is present.
	assert.True(t, contains(masterConfigMap(t, crName).Data["config.yaml"], "creds-v2"),
		"master ConfigMap should contain updated path_segment creds-v2")
}
