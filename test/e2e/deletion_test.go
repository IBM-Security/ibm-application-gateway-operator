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

// TestCRDeletionCleansUpOwnedResources creates a fresh CR, waits for its
// Deployment and master ConfigMap to appear, deletes the CR, then asserts that
// both the Deployment and ConfigMap are garbage-collected.
func TestCRDeletionCleansUpOwnedResources(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-delete-test"
	cr := buildIAGCR(crName, testNamespace, iagImage,
		"version: \"26.7\"\n")
	require.NoError(t, c.Create(context.Background(), cr))

	// Wait for the Deployment and master ConfigMap to exist before deleting.
	waitForDeployment(t, crName, shortTimeout)
	waitForMasterConfigMapContent(t, crName, "version", shortTimeout)

	// Delete the CR; owned resources should be garbage-collected.
	require.NoError(t, c.Delete(context.Background(), cr))

	// Deployment should disappear.
	waitForDeploymentGone(t, crName, shortTimeout)

	// Master ConfigMap (label app=<crName>) should also be gone.
	require.Eventually(t, func() bool {
		var cmList corev1.ConfigMapList
		if err := c.List(context.Background(), &cmList,
			client.InNamespace(testNamespace),
			client.MatchingLabels{"app": crName},
		); err != nil {
			return false
		}
		return len(cmList.Items) == 0
	}, shortTimeout, pollInterval,
		"master ConfigMap for %q was not garbage-collected after CR deletion", crName)

	// No ReplicaSets labelled app=<crName> should remain.
	require.Eventually(t, func() bool {
		var rsList appsv1.ReplicaSetList
		if err := c.List(context.Background(), &rsList,
			client.InNamespace(testNamespace),
			client.MatchingLabels{"app": crName},
		); err != nil {
			return false
		}
		return len(rsList.Items) == 0
	}, shortTimeout, pollInterval,
		"ReplicaSets for %q were not cleaned up after CR deletion", crName)

	assert.True(t, true, "all owned resources were cleaned up")
}
