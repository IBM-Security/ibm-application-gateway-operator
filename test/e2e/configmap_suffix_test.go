/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package e2e

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ibmv1 "github.com/ibm-security/ibm-application-gateway-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestCustomConfigMapSuffix verifies that setting spec.deployment.generatedConfigmapSuffix
// causes the operator to use the custom suffix when naming the master ConfigMap.
// The controller builds the GenerateName as: <crName><suffix> (via getConfigMapName).
// Default suffix is "-config-iag-internal-generated"; this test overrides it.
func TestCustomConfigMapSuffix(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-suffix-test"
	customSuffix := "-my-custom-suffix"

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
				ConfigMapSuffix: customSuffix,
			},
			Configuration: []ibmv1.IBMApplicationGatewayConfiguration{
				{Type: "literal", Value: "version: \"26.7\"\n"},
			},
		},
	}
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, crName, shortTimeout)

	// Fetch the master ConfigMap by label.
	var cmList corev1.ConfigMapList
	require.NoError(t, c.List(context.Background(), &cmList,
		client.InNamespace(testNamespace),
		client.MatchingLabels{"app": crName},
	))
	require.NotEmpty(t, cmList.Items,
		"master ConfigMap with label app=%q should exist", crName)

	cm := cmList.Items[0]

	// The controller sets GenerateName = crName + suffix.
	// The actual Name will be that prefix with a random suffix appended by k8s.
	// We assert the Name starts with the expected prefix.
	expectedPrefix := crName + customSuffix
	assert.True(t,
		strings.HasPrefix(cm.Name, expectedPrefix),
		"ConfigMap name %q should start with %q", cm.Name, expectedPrefix)
}
