/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// operatorNetPolName is the kustomize-expanded name of the egress NetworkPolicy:
// namePrefix (ibm-application-gateway-operator-) + name in manifest (controller-manager-egress).
const operatorNetPolName = "ibm-application-gateway-operator-controller-manager-egress"

// TestEgressNetworkPolicyExists verifies that the egress NetworkPolicy shipped
// in config/network-policy/operator-egress.yaml has been applied to the
// operator namespace.  The policy must:
//   - exist in the operator namespace
//   - select pods with label control-plane=controller-manager
//   - restrict only Egress traffic
//   - have exactly three egress rules:
//     1. DNS: UDP+TCP port 53 (any destination)
//     2. kube-system HTTPS: TCP 443 to kube-system namespace
//     3. External HTTPS: TCP 443 to 0.0.0.0/0 except 169.254.0.0/16
func TestEgressNetworkPolicyExists(t *testing.T) {
	var np networkingv1.NetworkPolicy
	require.NoError(t,
		c.Get(context.Background(),
			client.ObjectKey{Name: operatorNetPolName, Namespace: operatorNS}, &np),
		"NetworkPolicy %q must exist in namespace %q", operatorNetPolName, operatorNS)

	// Must be Egress-only.
	require.Len(t, np.Spec.PolicyTypes, 1,
		"NetworkPolicy should have exactly one PolicyType")
	assert.Equal(t, networkingv1.PolicyTypeEgress, np.Spec.PolicyTypes[0],
		"PolicyType must be Egress")

	// PodSelector must target the operator manager pods.
	labels := np.Spec.PodSelector.MatchLabels
	assert.Equal(t, "controller-manager", labels["control-plane"],
		"PodSelector should match control-plane=controller-manager")

	// Must have exactly 3 egress rules.
	require.Len(t, np.Spec.Egress, 3,
		"NetworkPolicy should have exactly 3 egress rules")

	// --- Rule 0: DNS (UDP + TCP port 53) ---
	dnsRule := np.Spec.Egress[0]
	assert.Empty(t, dnsRule.To,
		"DNS rule should have no destination (allow to any)")
	require.Len(t, dnsRule.Ports, 2,
		"DNS rule should have 2 port entries (UDP/53 and TCP/53)")

	var hasDNSUDP, hasDNSTCP bool
	for _, p := range dnsRule.Ports {
		portNum := p.Port.IntValue()
		if portNum == 53 {
			switch *p.Protocol {
			case "UDP":
				hasDNSUDP = true
			case "TCP":
				hasDNSTCP = true
			}
		}
	}
	assert.True(t, hasDNSUDP, "DNS rule must allow UDP/53")
	assert.True(t, hasDNSTCP, "DNS rule must allow TCP/53")

	// --- Rule 1: kube-system HTTPS (TCP 443) ---
	kubeRule := np.Spec.Egress[1]
	require.Len(t, kubeRule.To, 1,
		"kube-system rule should have exactly one To peer")
	require.NotNil(t, kubeRule.To[0].NamespaceSelector,
		"kube-system rule To peer must use a NamespaceSelector")
	assert.Equal(t,
		"kube-system",
		kubeRule.To[0].NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"],
		"kube-system rule must target the kube-system namespace")
	require.Len(t, kubeRule.Ports, 1)
	assert.Equal(t, 443, kubeRule.Ports[0].Port.IntValue(),
		"kube-system rule must allow port 443")

	// --- Rule 2: External HTTPS with link-local exclusion ---
	extRule := np.Spec.Egress[2]
	require.Len(t, extRule.To, 1,
		"external rule should have exactly one To peer")
	require.NotNil(t, extRule.To[0].IPBlock,
		"external rule To peer must use an IPBlock")
	assert.Equal(t, "0.0.0.0/0", extRule.To[0].IPBlock.CIDR,
		"external rule CIDR must be 0.0.0.0/0")
	require.Contains(t, extRule.To[0].IPBlock.Except, "169.254.0.0/16",
		"link-local 169.254.0.0/16 must be in the Except list")
	require.Len(t, extRule.Ports, 1)
	assert.Equal(t, 443, extRule.Ports[0].Port.IntValue(),
		"external rule must allow port 443")
}
