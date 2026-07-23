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

	ibmv1 "github.com/ibm-security/ibm-application-gateway-operator/api/v1"
)

// TestSplitConfigMergedInOrder verifies the README's "Split Configuration Example"
// scalar-overwrite rule: when two sources define the same key, the later source wins.
//
// CR has two literal sources:
//   - source 1: version: "26.6"
//   - source 2: version: "26.7"
//
// Expected merged output: version: "26.7"  (source 2 overwrites source 1)
func TestSplitConfigMergedInOrder(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-merge-scalar-test"
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
			// Two literal sources with conflicting version keys.
			// Controller merges in order; later entry overwrites earlier for scalars.
			Configuration: []ibmv1.IBMApplicationGatewayConfiguration{
				{
					Type:  "literal",
					Value: "version: \"26.6\"\n",
				},
				{
					Type:  "literal",
					Value: "version: \"26.7\"\n",
				},
			},
		},
	}
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, crName, shortTimeout)

	// The master ConfigMap must contain the later value (26.7), not the earlier (26.6).
	waitForMasterConfigMapContent(t, crName, "26.7", shortTimeout)
	cm := masterConfigMap(t, crName)
	assert.True(t, contains(cm.Data["config.yaml"], "26.7"),
		"merged ConfigMap should contain version 26.7 (later source wins)")
	assert.False(t, contains(cm.Data["config.yaml"], "26.6") &&
		!contains(cm.Data["config.yaml"], "26.7"),
		"merged ConfigMap must not contain only the earlier version 26.6")
}

// TestSplitConfigArraysConcatenated verifies the README's array-concatenation rule:
// when two sources each define entries in the same array, the merged output contains
// all entries from both sources.
//
// CR has two literal sources each defining one resource_server entry.
// Expected: merged ConfigMap contains paths from BOTH sources.
func TestSplitConfigArraysConcatenated(t *testing.T) {
	ensureNamespace(t)
	ensureServiceAccount(t)

	crName := "iag-merge-array-test"
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
			Configuration: []ibmv1.IBMApplicationGatewayConfiguration{
				{
					Type: "literal",
					Value: `version: "26.7"
resource_servers:
  - path: /app1
    connection_type: tcp
    servers:
      - host: 10.0.0.1
        port: 8080
`,
				},
				{
					Type: "literal",
					Value: `resource_servers:
  - path: /app2
    connection_type: tcp
    servers:
      - host: 10.0.0.2
        port: 8081
`,
				},
			},
		},
	}
	require.NoError(t, c.Create(context.Background(), cr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), cr) })

	waitForDeployment(t, crName, shortTimeout)
	waitForMasterConfigMapContent(t, crName, "resource_servers", shortTimeout)

	// Both resource server paths must appear in the merged ConfigMap.
	cm := masterConfigMap(t, crName)
	assert.True(t, contains(cm.Data["config.yaml"], "/app1"),
		"merged ConfigMap should contain /app1 from first source")
	assert.True(t, contains(cm.Data["config.yaml"], "/app2"),
		"merged ConfigMap should contain /app2 from second source")
}
