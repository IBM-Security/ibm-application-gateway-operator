package controller

import (
	"github.com/ibm/ibm-application-gateway-operator/pkg/controller/ibmapplicationgateway"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, ibmapplicationgateway.Add)
}
