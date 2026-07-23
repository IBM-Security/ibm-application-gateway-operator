/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ibmv1 "github.com/ibm-security/ibm-application-gateway-operator/api/v1"
)

const (
	pollInterval = 3 * time.Second
	shortTimeout = 2 * time.Minute
	longTimeout  = 5 * time.Minute
)

// ensureNamespace creates the test namespace if it does not already exist.
// A Cleanup is registered to delete the namespace when the test ends.
func ensureNamespace(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
	_ = c.Create(ctx, ns) // ignore AlreadyExists
	t.Cleanup(func() {
		_ = c.Delete(context.Background(), ns)
	})
}

// ensureServiceAccount creates the "iag" ServiceAccount used by test CRs.
// Errors are intentionally ignored — the SA may already exist.
func ensureServiceAccount(t *testing.T) {
	t.Helper()
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "iag", Namespace: testNamespace},
	}
	_ = c.Create(context.Background(), sa)
}

// waitForDeployment polls until the named Deployment exists in testNamespace.
func waitForDeployment(t *testing.T, name string, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		var d appsv1.Deployment
		return c.Get(context.Background(),
			client.ObjectKey{Name: name, Namespace: testNamespace}, &d) == nil
	}, timeout, pollInterval, "deployment %q was not created within %s", name, timeout)
}

// waitForDeploymentGone polls until the named Deployment is absent from testNamespace.
func waitForDeploymentGone(t *testing.T, name string, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		var d appsv1.Deployment
		err := c.Get(context.Background(),
			client.ObjectKey{Name: name, Namespace: testNamespace}, &d)
		return errors.IsNotFound(err)
	}, timeout, pollInterval, "deployment %q was not deleted within %s", name, timeout)
}

// masterConfigMap returns the operator-generated merged ConfigMap for a CR.
// It is found by the label app=<crName>, which the controller always sets via
// getNewConfigMap in ibmapplicationgateway_controller.go.
func masterConfigMap(t *testing.T, crName string) *corev1.ConfigMap {
	t.Helper()
	var cmList corev1.ConfigMapList
	require.NoError(t, c.List(context.Background(), &cmList,
		client.InNamespace(testNamespace),
		client.MatchingLabels{"app": crName},
	))
	require.NotEmpty(t, cmList.Items, "no master ConfigMap found for CR %q", crName)
	return &cmList.Items[0]
}

// waitForMasterConfigMapContent polls until the master ConfigMap for crName
// contains the given substring in its config.yaml data key.
func waitForMasterConfigMapContent(t *testing.T, crName, substring string, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		var cmList corev1.ConfigMapList
		if err := c.List(context.Background(), &cmList,
			client.InNamespace(testNamespace),
			client.MatchingLabels{"app": crName},
		); err != nil {
			return false
		}
		if len(cmList.Items) == 0 {
			return false
		}
		data := cmList.Items[0].Data["config.yaml"]
		return len(data) > 0 && contains(data, substring)
	}, timeout, pollInterval,
		"master ConfigMap for %q did not contain %q within %s", crName, substring, timeout)
}

// getDeployment returns the named Deployment from testNamespace, failing the
// test if the Deployment cannot be retrieved.
func getDeployment(t *testing.T, name string) appsv1.Deployment {
	t.Helper()
	var d appsv1.Deployment
	require.NoError(t, c.Get(context.Background(),
		client.ObjectKey{Name: name, Namespace: testNamespace}, &d),
		"failed to get Deployment %q", name)
	return d
}

// buildIAGCR constructs a minimal IBMApplicationGateway CR with a single
// literal configuration entry. It does NOT create it in the cluster.
func buildIAGCR(name, namespace, image, literalConfig string) *ibmv1.IBMApplicationGateway {
	replicas := int32(1)
	return &ibmv1.IBMApplicationGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ibmv1.IBMApplicationGatewaySpec{
			Replicas: replicas,
			Deployment: ibmv1.IBMApplicationGatewayDeployment{
				ImageLocation:   image,
				ImagePullPolicy: "IfNotPresent",
				Lang:            "C",
			},
			Configuration: []ibmv1.IBMApplicationGatewayConfiguration{
				{
					Type:  "literal",
					Value: literalConfig,
				},
			},
		},
	}
}

// contains is a simple helper that avoids importing strings in every test file.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
