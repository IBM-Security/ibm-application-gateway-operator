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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// deploymentReplicas returns the replica count of the named Deployment, or -1
// on any error (used inside require.Eventually callbacks).
func deploymentReplicas(name string) int32 {
	var d appsv1.Deployment
	if err := c.Get(context.Background(),
		client.ObjectKey{Name: name, Namespace: testNamespace}, &d); err != nil {
		return -1
	}
	if d.Spec.Replicas == nil {
		return -1
	}
	return *d.Spec.Replicas
}

// TestReplicaScaleUp creates an IAG CR with 1 replica, patches it to 3, and
// asserts the Deployment reaches the new count.
func TestReplicaScaleUp(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-scale-test"
	cr := buildIAGCR(crName, testNamespace, iagImage,
		"version: \"26.7\"\n")
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, crName, shortTimeout)

	// Re-fetch to get current ResourceVersion before patching.
	require.NoError(t, c.Get(context.Background(),
		client.ObjectKey{Name: crName, Namespace: testNamespace}, cr))
	cr.Spec.Replicas = 3
	require.NoError(t, c.Update(context.Background(), cr))

	require.Eventually(t, func() bool {
		return deploymentReplicas(crName) == 3
	}, shortTimeout, pollInterval, "Deployment %q did not scale up to 3 replicas", crName)

	assert.Equal(t, int32(3), deploymentReplicas(crName))
}

// TestReplicaScaleDown creates a CR at 3 replicas, scales it to 1, and asserts
// the Deployment is updated. Also verifies that kubernetes.io/change-cause is
// NOT set for a replica-only change (per controller behaviour).
func TestReplicaScaleDown(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-scaledown-test"
	cr := buildIAGCR(crName, testNamespace, iagImage,
		"version: \"26.7\"\n")
	cr.Spec.Replicas = 3
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, crName, shortTimeout)
	require.Eventually(t, func() bool {
		return deploymentReplicas(crName) == 3
	}, shortTimeout, pollInterval, "Deployment %q did not reach 3 replicas", crName)

	// Re-fetch to get latest ResourceVersion before patching.
	require.NoError(t, c.Get(context.Background(),
		client.ObjectKey{Name: crName, Namespace: testNamespace}, cr))
	cr.Spec.Replicas = 1
	require.NoError(t, c.Update(context.Background(), cr))

	require.Eventually(t, func() bool {
		return deploymentReplicas(crName) == 1
	}, shortTimeout, pollInterval, "Deployment %q did not scale down to 1 replica", crName)

	// Replica-only changes must NOT set kubernetes.io/change-cause on the Deployment.
	d := getDeployment(t, crName)
	assert.Empty(t, d.Annotations["kubernetes.io/change-cause"],
		"replica-only change must not set kubernetes.io/change-cause")
}
