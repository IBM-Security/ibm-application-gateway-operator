/*
 * Copyright contributors to the IBM Application Gateway Operator project
 */

package e2e

import (
	"flag"
	"os"
	"testing"

	ibmv1 "github.com/ibm-security/ibm-application-gateway-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	c             client.Client
	testNamespace string
	operatorNS    string
	iagImage      string
	iagAltImage   string
)

func TestMain(m *testing.M) {
	flag.StringVar(&testNamespace, "test-namespace", "ivia-e2e", "namespace for test CRs")
	flag.StringVar(&operatorNS, "namespace", "operators", "namespace the operator is installed in")
	flag.StringVar(&iagImage, "iag-image",
		"icr.io/ibmappgateway/ibm-application-gateway:latest",
		"IAG container image used in test CRs")
	flag.StringVar(&iagAltImage, "iag-alt-image",
		"icr.io/ibmappgateway/ibm-application-gateway:26.6.0",
		"alternative IAG image used by image-change tests")
	flag.Parse()

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = ibmv1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)

	cfg, err := config.GetConfig()
	if err != nil {
		panic("cannot get kubeconfig: " + err.Error())
	}

	c, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		panic("cannot create client: " + err.Error())
	}

	os.Exit(m.Run())
}
