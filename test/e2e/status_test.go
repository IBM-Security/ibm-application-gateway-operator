/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ibmv1 "github.com/ibm-security/ibm-application-gateway-operator/api/v1"
)

// TestCRStatusFalseOnBadConfigMap creates an IBMApplicationGateway CR that
// references a ConfigMap which does not exist. The controller's manageError()
// path sets Status.Status = false. This test asserts that the status subresource
// is written correctly when reconcile fails.
func TestCRStatusFalseOnBadConfigMap(t *testing.T) {
	ensureNamespace(t)

	crName := "iag-bad-cm-test"
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
			// Reference a ConfigMap that does not exist — reconcile must fail.
			Configuration: []ibmv1.IBMApplicationGatewayConfiguration{
				{
					Type:    "configmap",
					Name:    "nonexistent-configmap-xyz",
					DataKey: "config.yaml",
				},
			},
		},
	}
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	// Wait for the controller to attempt reconciliation and set Status.Status = false.
	require.Eventually(t, func() bool {
		var latest ibmv1.IBMApplicationGateway
		if err := c.Get(context.Background(),
			client.ObjectKey{Name: crName, Namespace: testNamespace}, &latest); err != nil {
			return false
		}
		return !latest.Status.Status
	}, shortTimeout, pollInterval,
		"CR %q Status.Status should become false after failed reconcile", crName)

	var latest ibmv1.IBMApplicationGateway
	require.NoError(t, c.Get(context.Background(),
		client.ObjectKey{Name: crName, Namespace: testNamespace}, &latest))
	assert.False(t, latest.Status.Status,
		"CR Status.Status must be false when the referenced ConfigMap does not exist")
}
