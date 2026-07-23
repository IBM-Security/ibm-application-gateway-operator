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

// TestImageChangeRollingUpdate patches the CR's image and asserts that the
// Deployment template container image is updated and the change-cause annotation
// is set on the Deployment.
func TestImageChangeRollingUpdate(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-image-test"
	cr := buildIAGCR(crName, testNamespace, iagImage, "version: \"26.7\"\n")
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, crName, shortTimeout)

	// Verify the initial image is set correctly.
	d := getDeployment(t, crName)
	require.Equal(t, iagImage, d.Spec.Template.Spec.Containers[0].Image,
		"initial container image should match CR spec")

	// Re-fetch CR to get current ResourceVersion before patching.
	require.NoError(t, c.Get(context.Background(),
		client.ObjectKey{Name: crName, Namespace: testNamespace}, cr))
	cr.Spec.Deployment.ImageLocation = iagAltImage
	require.NoError(t, c.Update(context.Background(), cr))

	// Wait for the Deployment container image to be updated.
	require.Eventually(t, func() bool {
		var dep appsv1.Deployment
		if err := c.Get(context.Background(),
			client.ObjectKey{Name: crName, Namespace: testNamespace}, &dep); err != nil {
			return false
		}
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			return false
		}
		return dep.Spec.Template.Spec.Containers[0].Image == iagAltImage
	}, longTimeout, pollInterval,
		"Deployment container image was not updated to %q", iagAltImage)

	// Assert the Deployment annotation records the reason for the rollout.
	d = getDeployment(t, crName)
	assert.True(t,
		contains(d.Annotations["kubernetes.io/change-cause"], "Image changed"),
		"Deployment annotation kubernetes.io/change-cause should contain 'Image changed', got %q",
		d.Annotations["kubernetes.io/change-cause"],
	)

	// Restore the original image in Cleanup so other tests are not affected.
	t.Cleanup(func() {
		var crLatest = cr.DeepCopy()
		if err := c.Get(context.Background(),
			client.ObjectKey{Name: crName, Namespace: testNamespace}, crLatest); err != nil {
			return
		}
		crLatest.Spec.Deployment.ImageLocation = iagImage
		_ = c.Update(context.Background(), crLatest)
	})
}
