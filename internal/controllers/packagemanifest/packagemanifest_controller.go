package packagemanifest

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	batchv1 "k8s.io/api/batch/v1"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
)

// PackageManifest reconciler
type PackageManifestController struct {
	client client.Client
	log    logr.Logger
	scheme *runtime.Scheme
	dynamicCache    dynamicCache
}

type dynamicCache interface {
	client.Reader
	Source() source.Source
	Free(ctx context.Context, obj client.Object) error
	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
}

func (r *PackageManifestController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.PackageManifest{}).
		Owns(&corev1alpha1.ObjectSet{}). // change this to ObjectDeployment after ObjectDeployment is present
		//Owns(&batchv1.Job{}). no need to own the job
		Complete(r)
}

func (r *PackageManifestController) ensureDynamicCacheLabel(ctx context.Context, packageManifest *corev1alpha1.PackageManifest) error {
	labels := packageManifest.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	if labels[controllers.DynamicCacheLabel] == "True" {
		return nil
	}
	labels[controllers.DynamicCacheLabel] = "True"
	packageManifest.Labels = labels
	return r.client.Update(ctx, packageManifest)
}

func (r *PackageManifestController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := r.log.WithValues("PackageManifest", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)

	packageManifest := corev1alpha1.PackageManifest{}
	if err := r.client.Get(
		ctx, req.NamespacedName, &packageManifest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.ensureDynamicCacheLabel(ctx, &packageManifest); err != nil {
		return ctrl.Result{}, fmt.Errorf("error occurred while ensuring the dynamic cache label: %w", err)
	}

	if !packageManifest.GetDeletionTimestamp().IsZero() {
		for finalizer, cleaner := range FinalizersToCleaners {
			if err := cleaner(r.client, &packageManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("error occurred while cleaning up the '%s' finalizer of PackageManifest: %w", finalizer, err)
			}
			if err := controllers.RemoveFinalizers(ctx, r.client, &packageManifest, string(finalizer)); err != nil {
				return ctrl.Result{}, fmt.Errorf("error occurred while removing the '%s' finalizer from the PackageManifest: %w", finalizer, err)
			}
		}
		return ctrl.Result{}, nil
	}

	if err := controllers.EnsureFinalizers(ctx, r.client, &packageManifest, packageManifestFinalizers()...); err != nil {
		return ctrl.Result{}, err
	}

	if err := controllers.EnsureCachedFinalizer(ctx, r.client, &packageManifest); err != nil {
		return ctrl.Result{}, err
	}

	packageName, packageBundleImage := packageManifest.Name, packageManifest.Spec.PackageImage
	packageLoaderJob, err := jobNameFromPackageBundleImage(packageName, packageBundleImage)
	if err != nil {
		// report status
		return ctrl.Result{}, err
	}

	desiredJob := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      packageLoaderJob,
			Namespace: packageManifest.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "package-loader",
							Image:   "quay.io/mt-sre/package-loader",
							Command: []string{"package-loader"},
							Args:    []string{"--namespace", packageManifest.Namespace, "--package-name", packageName},
						},
					},
				},
			},
		},
	}
	var foundJob batchv1.Job
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(&desiredJob), &foundJob); err != nil {
		if errors.IsNotFound(err) {
			if err := r.client.Create(ctx, &desiredJob); err != nil {
				// status: failed to create the package-loader job
				return ctrl.Result{}, err
			}
			// status: created the package-loader job
		}
		// status: failed to get check for existing package-loader jobs
		return ctrl.Result{}, err
	}

	for _, condition := range foundJob.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			// status: success of the package manifest
			break
		}
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			// status : failure - job failed. we shouldn't feel the need of retrying the job as it's the responsibility of the job cr to perform retries
			break
		}
		// else just stay stuck on the pending state
	}

	return ctrl.Result{}, nil
}

func jobNameFromPackageBundleImage(packageName string, packageBundleImage string) (string, error) {
	colonTokenizedImage := strings.Split(packageBundleImage, ":")
	if len(colonTokenizedImage) == 1 {
		return "", fmt.Errorf("image not found to be in the right format with digest")
	}
	digest := colonTokenizedImage[len(colonTokenizedImage)-1]
	return fmt.Sprintf("job-%s-%s", packageName, digest), nil
}