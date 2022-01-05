/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"

	ibmv1 "github.com/ibm-security/ibm-application-gateway-operator/api/v1"
)

// IBMApplicationGatewayReconciler reconciles a IBMApplicationGateway object
type IBMApplicationGatewayReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Logger
var log = logf.Log.WithName("controller_ibmapplicationgateway")

//+kubebuilder:rbac:groups=ibm.com,resources=ibmapplicationgateways,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ibm.com,resources=ibmapplicationgateways/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ibm.com,resources=ibmapplicationgateways/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the IBMApplicationGateway object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *IBMApplicationGatewayReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

        reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling IBMApplicationGateway")

	// your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IBMApplicationGatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ibmv1.IBMApplicationGateway{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
