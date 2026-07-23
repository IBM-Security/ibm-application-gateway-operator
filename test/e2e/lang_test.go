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

// langLabelKeyE2E mirrors the controller constant so tests do not import internal packages.
const langLabelKeyE2E = "ibm-application-gateway.operator.security.ibm.com/lang"

// TestLanguageChangeAnnotatesRollout patches spec.deployment.lang and asserts that
// the controller updates:
//   - the pod template LANG env var
//   - the pod template label ibm-application-gateway.operator.security.ibm.com/lang
//   - Deployment.Annotations["kubernetes.io/change-cause"] (contains "Language changed")
func TestLanguageChangeAnnotatesRollout(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-lang-test"
	cr := buildIAGCR(crName, testNamespace, iagImage,
		"version: \"26.7\"\n")
	// Start with the default language (C / English).
	cr.Spec.Deployment.Lang = "C"
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, crName, shortTimeout)

	// Confirm the initial language label on the pod template.
	d := getDeployment(t, crName)
	assert.Equal(t, "C", d.Spec.Template.Labels[langLabelKeyE2E],
		"initial pod template label should be lang=C")

	// Patch the CR to use French.
	require.NoError(t, c.Get(context.Background(),
		client.ObjectKey{Name: crName, Namespace: testNamespace}, cr))
	cr.Spec.Deployment.Lang = "fr"
	require.NoError(t, c.Update(context.Background(), cr))

	// Wait for the Deployment template label to reflect the new language.
	require.Eventually(t, func() bool {
		var dep appsv1.Deployment
		if err := c.Get(context.Background(),
			client.ObjectKey{Name: crName, Namespace: testNamespace}, &dep); err != nil {
			return false
		}
		return dep.Spec.Template.Labels[langLabelKeyE2E] == "fr"
	}, shortTimeout, pollInterval,
		"pod template label %s was not updated to 'fr'", langLabelKeyE2E)

	d = getDeployment(t, crName)

	// LANG env var must be set to "fr" in the container spec.
	var foundLang string
	for _, env := range d.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "LANG" {
			foundLang = env.Value
		}
	}
	assert.Equal(t, "fr", foundLang,
		"container LANG env var should be updated to fr")

	// change-cause must contain "Language changed".
	assert.True(t,
		contains(d.Annotations["kubernetes.io/change-cause"], "Language changed"),
		"change-cause annotation should contain 'Language changed', got: %q",
		d.Annotations["kubernetes.io/change-cause"])
}
