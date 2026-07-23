/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeploymentCreated verifies that creating an IBMApplicationGateway CR
// causes the controller to create a Deployment and a master ConfigMap.
func TestDeploymentCreated(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-deploy-test"
	literalConfig := `
version: "26.7"
server:
  local_applications:
    cred_viewer:
      path_segment: "creds"
      enable_html: true
`
	cr := buildIAGCR(crName, testNamespace, iagImage, literalConfig)
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() {
		_ = c.Delete(context.Background(), cr)
	})

	// Wait for the Deployment to be created.
	waitForDeployment(t, crName, shortTimeout)

	// Assert the Deployment has the expected replica count.
	d := getDeployment(t, crName)
	require.NotNil(t, d.Spec.Replicas, "Deployment.Spec.Replicas should not be nil")
	assert.Equal(t, int32(1), *d.Spec.Replicas, "expected 1 replica")

	// Assert the master ConfigMap was created and contains the config.
	cm := masterConfigMap(t, crName)
	assert.True(t, len(cm.Data["config.yaml"]) > 0, "master ConfigMap config.yaml should not be empty")

	// ConfigMap is owned by the CR so it will be GC'd; delete here to speed up
	// namespace cleanup.
	_ = c.Delete(context.Background(), cm)
}

// TestDeploymentLabels verifies that the created Deployment carries the
// app=<crName> selector label that the operator always sets.
func TestDeploymentLabels(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-labels-test"
	cr := buildIAGCR(crName, testNamespace, iagImage,
		"version: \"26.7\"\n")
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() {
		_ = c.Delete(context.Background(), cr)
	})

	waitForDeployment(t, crName, shortTimeout)

	d := getDeployment(t, crName)
	assert.Equal(t, crName, d.Labels["app"],
		"Deployment should carry label app=<crName>")
	assert.Equal(t, crName, d.Spec.Selector.MatchLabels["app"],
		"Deployment selector should match app=<crName>")
}
