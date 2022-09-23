package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// This label is set on all dynamic objects to limit caches.
	DynamicCacheLabel = "package-operator.run/cache"
	// Common finalizer to free allocated caches when objects are deleted.
	CachedFinalizer = "package-operator.run/cached"
)

// Ensures the given finalizers are set and persisted on the given object.
func EnsureFinalizers(
	ctx context.Context, c client.Client,
	obj client.Object, finalizers ...string,
) error {
	noFinalizersToPatch := true
	for _, finalizer := range finalizers {
		if controllerutil.ContainsFinalizer(obj, finalizer) {
			continue
		}
		noFinalizersToPatch = false
		controllerutil.AddFinalizer(obj, finalizer)
	}
	if noFinalizersToPatch {
		return nil
	}

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"resourceVersion": obj.GetResourceVersion(),
			"finalizers":      obj.GetFinalizers(),
		},
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshalling patch to remove finalizer: %w", err)
	}

	if err := c.Patch(ctx, obj, client.RawPatch(types.MergePatchType, patchJSON)); err != nil {
		return fmt.Errorf("adding finalizer: %w", err)
	}
	return nil
}

// Removes the given finalizers and persist the change.
func RemoveFinalizers(
	ctx context.Context, c client.Client,
	obj client.Object, finalizers ...string,
) error {
	noFinalizersToPatch := true
	for _, finalizer := range finalizers {
		if !controllerutil.ContainsFinalizer(obj, finalizer) {
			continue
		}
		noFinalizersToPatch = false
		controllerutil.RemoveFinalizer(obj, finalizer)
	}
	if noFinalizersToPatch {
		return nil
	}

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"resourceVersion": obj.GetResourceVersion(),
			"finalizers":      obj.GetFinalizers(),
		},
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshalling patch to remove finalizer: %w", err)
	}
	if err := c.Patch(ctx, obj, client.RawPatch(types.MergePatchType, patchJSON)); err != nil {
		return fmt.Errorf("removing finalizer: %w", err)
	}
	return nil
}

func EnsureCachedFinalizer(
	ctx context.Context, c client.Client, obj client.Object,
) error {
	return EnsureFinalizers(ctx, c, obj, CachedFinalizer)
}

type cacheFreer interface {
	Free(ctx context.Context, obj client.Object) error
}

// Frees caches and removes the associated finalizer.
func FreeCacheAndRemoveFinalizer(
	ctx context.Context, c client.Client,
	obj client.Object, cache cacheFreer,
) error {
	if err := cache.Free(ctx, obj); err != nil {
		return fmt.Errorf("free cache: %w", err)
	}

	return RemoveFinalizers(ctx, c, obj, CachedFinalizer)
}
