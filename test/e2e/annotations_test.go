/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ibmv1 "github.com/ibm-security/ibm-application-gateway-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestILMTAnnotationsPresent asserts that the operator always stamps the IBM
// License Metric Tool (ILMT) annotations onto the managed pod template.
// These are required for IBM product compliance and are always applied by
// newDeploymentForCR regardless of the licenseAnnotation field value.
func TestILMTAnnotationsPresent(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-ilmt-test"
	cr := buildIAGCR(crName, testNamespace, iagImage,
		"version: \"26.7\"\n")
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, crName, shortTimeout)

	d := getDeployment(t, crName)
	podAnnotations := d.Spec.Template.Annotations

	assert.Equal(t, "IBM Application Gateway", podAnnotations["productName"],
		"ILMT productName must be set on pod template")
	assert.Equal(t, "7c3292bae26f4f699486bc4b8d05166c", podAnnotations["productId"],
		"ILMT productId must be set on pod template")
	assert.Equal(t, "PROCESSOR_VALUE_UNIT", podAnnotations["productMetric"],
		"ILMT productMetric must be set on pod template")
	assert.Equal(t, "All", podAnnotations["productChargedContainers"],
		"ILMT productChargedContainers must be set on pod template")
}

// TestCustomAnnotationsForwarded asserts that custom annotations defined in
// spec.deployment.customAnnotations are added to the pod template by the operator.
func TestCustomAnnotationsForwarded(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-custom-annot-test"
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
				CustomAnnotations: []ibmv1.CustomAnnotation{
					{Key: "my-org/owner", Value: "team-alpha"},
					{Key: "my-org/env", Value: "e2e-test"},
				},
			},
			Configuration: []ibmv1.IBMApplicationGatewayConfiguration{
				{Type: "literal", Value: "version: \"26.7\"\n"},
			},
		},
	}
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, crName, shortTimeout)

	d := getDeployment(t, crName)
	podAnnotations := d.Spec.Template.Annotations

	assert.Equal(t, "team-alpha", podAnnotations["my-org/owner"],
		"custom annotation my-org/owner must be present on pod template")
	assert.Equal(t, "e2e-test", podAnnotations["my-org/env"],
		"custom annotation my-org/env must be present on pod template")
}
