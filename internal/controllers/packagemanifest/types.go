package packagemanifest

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Finalizer string
type Cleaner func(c client.Client, obj client.Object) error


const ObjectDeploymentFinalizer Finalizer = "package-operator.run/package-manifest/object-deployment"


var FinalizersToCleaners = map[Finalizer]Cleaner{
	ObjectDeploymentFinalizer: func(c client.Client, obj client.Object) error {
		// TODO: determine the object deployment name and delete it
		return nil
	},
}

func packageManifestFinalizers() []string {
	res := []string{}
	for finalizer := range FinalizersToCleaners {
		res = append(res, string(finalizer))
	}
	return res
}