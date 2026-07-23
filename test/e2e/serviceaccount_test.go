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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestServiceAccountChangeAnnotatesRollout creates a CR with serviceAccountName="iag",
// then patches it to a second SA and asserts that the Deployment template
// ServiceAccountName is updated and change-cause records the reason.
func TestServiceAccountChangeAnnotatesRollout(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t) // creates "iag" SA

	// Create a second service account for the patch target.
	altSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "iag-alt", Namespace: testNamespace},
	}
	_ = c.Create(context.Background(), altSA) // ignore AlreadyExists
	t.Cleanup(func() { _ = c.Delete(context.Background(), altSA) })

	crName := "iag-sa-test"
	cr := buildIAGCR(crName, testNamespace, iagImage,
		"version: \"26.7\"\n")
	cr.Spec.Deployment.ServiceAccountName = "iag"
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, crName, shortTimeout)

	// Confirm initial service account.
	d := getDeployment(t, crName)
	assert.Equal(t, "iag", d.Spec.Template.Spec.ServiceAccountName,
		"initial Deployment should use ServiceAccountName=iag")

	// Patch CR to use the alternate SA.
	require.NoError(t, c.Get(context.Background(),
		client.ObjectKey{Name: crName, Namespace: testNamespace}, cr))
	cr.Spec.Deployment.ServiceAccountName = "iag-alt"
	require.NoError(t, c.Update(context.Background(), cr))

	// Wait for the Deployment to reflect the new SA.
	require.Eventually(t, func() bool {
		var dep appsv1.Deployment
		if err := c.Get(context.Background(),
			client.ObjectKey{Name: crName, Namespace: testNamespace}, &dep); err != nil {
			return false
		}
		return dep.Spec.Template.Spec.ServiceAccountName == "iag-alt"
	}, shortTimeout, pollInterval,
		"Deployment ServiceAccountName was not updated to 'iag-alt'")

	d = getDeployment(t, crName)
	assert.Equal(t, "iag-alt", d.Spec.Template.Spec.ServiceAccountName)

	// change-cause must contain "Service account changed".
	assert.True(t,
		contains(d.Annotations["kubernetes.io/change-cause"], "Service account changed"),
		"change-cause annotation should contain 'Service account changed', got: %q",
		d.Annotations["kubernetes.io/change-cause"])
}
