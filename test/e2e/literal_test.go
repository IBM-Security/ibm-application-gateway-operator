/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestLiteralConfigChangeRegeneratesConfigMap patches the CR's literal config and
// asserts that the master ConfigMap is updated (ResourceVersion changes).
func TestLiteralConfigChangeRegeneratesConfigMap(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-literal-test"
	initialConfig := `
version: "26.7"
server:
  local_applications:
    cred_viewer:
      path_segment: "creds-initial"
      enable_html: true
`
	cr := buildIAGCR(crName, testNamespace, iagImage,
		initialConfig)
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, crName, shortTimeout)
	waitForMasterConfigMapContent(t, crName, "creds-initial", shortTimeout)

	// Record the initial ConfigMap ResourceVersion.
	initialCM := masterConfigMap(t, crName)
	initialRV := initialCM.ResourceVersion

	// Patch the CR literal config to use a new path_segment.
	updatedConfig := `
version: "26.7"
server:
  local_applications:
    cred_viewer:
      path_segment: "creds-updated"
      enable_html: true
`
	require.NoError(t, c.Get(context.Background(),
		client.ObjectKey{Name: crName, Namespace: testNamespace}, cr))
	cr.Spec.Configuration[0].Value = updatedConfig
	require.NoError(t, c.Update(context.Background(), cr))

	// Wait for the master ConfigMap to change.
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
		"master ConfigMap for %q was not updated after literal config change", crName)

	// Assert the new content is present.
	cm := masterConfigMap(t, crName)
	assert.True(t, contains(cm.Data["config.yaml"], "creds-updated"),
		"master ConfigMap should contain the updated path_segment creds-updated")
}

// TestLiteralConfigChangeAnnotatesRollout asserts that a literal config change
// sets kubernetes.io/change-cause = "Configuration change" on the Deployment.
func TestLiteralConfigChangeAnnotatesRollout(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-annot-test"
	initialConfig := "version: \"26.7\"\n"
	cr := buildIAGCR(crName, testNamespace, iagImage,
		initialConfig)
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, crName, shortTimeout)

	// Apply a new literal config.
	updatedConfig := `
version: "26.7"
server:
  local_applications:
    cred_viewer:
      path_segment: "creds-annot"
      enable_html: true
`
	require.NoError(t, c.Get(context.Background(),
		client.ObjectKey{Name: crName, Namespace: testNamespace}, cr))
	cr.Spec.Configuration[0].Value = updatedConfig
	require.NoError(t, c.Update(context.Background(), cr))

	// Wait for the Deployment to carry the expected change-cause annotation.
	require.Eventually(t, func() bool {
		var dep appsv1.Deployment
		if err := c.Get(context.Background(),
			client.ObjectKey{Name: crName, Namespace: testNamespace}, &dep); err != nil {
			return false
		}
		return dep.Annotations["kubernetes.io/change-cause"] == "Configuration change"
	}, shortTimeout, pollInterval,
		"Deployment %q did not get change-cause=Configuration change", crName)
}
