// Copyright 2020 Tetrate
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	wasmxdsv1alpha1 "github.com/tetratelabs/wasmxds/api/v1alpha1"
	"github.com/tetratelabs/wasmxds/wasmxds"
)

// WasmExtensionReconciler reconciles a WasmExtension object
type WasmExtensionReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	eventHandler wasmxds.EventHandler
}

const wasmFilterFinalizer = "finalizer.wasmxds.tetrate.io"

func (r *WasmExtensionReconciler) SetEventHandler(handler wasmxds.EventHandler) {
	r.eventHandler = handler
}

// +kubebuilder:rbac:groups=wasmxds.tetrate.io,resources=wasmextensions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wasmxds.tetrate.io,resources=wasmextensions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get;list;watch

func (r *WasmExtensionReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("WasmExtension", req.NamespacedName)

	ext := &wasmxdsv1alpha1.WasmExtension{}
	err := r.Get(ctx, req.NamespacedName, ext)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("object already deleted", "name", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "failed to get object", "name", req.NamespacedName)
		return ctrl.Result{}, err
	}

	if ext.GetDeletionTimestamp() != nil {
		r.Log.Info("deleting filter", "name", req.NamespacedName)
		r.eventHandler.Delete(ext)
		r.Log.Info("remove finalizer", "name", req.NamespacedName)
		controllerutil.RemoveFinalizer(ext, wasmFilterFinalizer)
		if err := r.Update(ctx, ext); err != nil {
			r.Log.Error(err, "failed to set finalizer", "name", req.NamespacedName)
		}

		return ctrl.Result{}, nil
	}

	pc, vc, err := r.resolveConfigs(ext)
	if err != nil {
		r.Log.Error(err, "resolve configurations", "name", req.NamespacedName)
		return ctrl.Result{}, err
	}

	if !contains(ext.GetFinalizers(), wasmFilterFinalizer) {
		r.Log.Info("adding finalizer", "name", req.NamespacedName)
		controllerutil.AddFinalizer(ext, wasmFilterFinalizer)
		if err := r.Update(ctx, ext); err != nil {
			r.Log.Error(err, "failed to set finalizer", "name", req.NamespacedName)
		}
	}
	return r.eventHandler.Update(ext, pc, vc)
}

func (r *WasmExtensionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wasmxdsv1alpha1.WasmExtension{}).
		WithOptions(controller.Options{
			// TODO: support/verify concurrent access to registry
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

func (r *WasmExtensionReconciler) resolveConfigs(
	extension *wasmxdsv1alpha1.WasmExtension) (pluginConfig, vmConfig string, err error) {
	if extension.Spec.PluginConfiguration != nil {
		r.Log.Info("resolving plugin configuration", "name", extension.Namespaced())
		pluginConfig, err = r.resolveConfig(extension.Spec.PluginConfiguration)
		if err != nil {
			err = fmt.Errorf("failed to resolve plugin configuration: %w", err)
			return
		}
		r.Log.Info("plugin configuration resolved", "name", extension.Namespaced())
	}

	if extension.Spec.VMConfiguration != nil {
		r.Log.Info("resolving vm configuration", "name", extension.Namespaced())
		vmConfig, err = r.resolveConfig(extension.Spec.VMConfiguration)
		if err != nil {
			err = fmt.Errorf("failed to resolve vm configuration: %w", err)
		}
		r.Log.Info("vm configuration resolved", "name", extension.Namespaced())
	}
	return
}

func (r *WasmExtensionReconciler) resolveConfig(cv *wasmxdsv1alpha1.WasmExtensionConfigValue) (string, error) {
	if cv.Value != nil {
		return *cv.Value, nil
	}

	if cv.ValueFrom == nil {
		return "", fmt.Errorf("one of valueFrom and value must be set")
	}

	if cv.ValueFrom.ConfigMapKeyRef != nil {
		ns := types.NamespacedName{
			Namespace: cv.ValueFrom.ConfigMapKeyRef.Namespace,
			Name:      cv.ValueFrom.ConfigMapKeyRef.Name,
		}
		var cm v1.ConfigMap
		if err := r.Client.Get(context.Background(), ns, &cm); err != nil {
			return "", fmt.Errorf("error getting configmap %s: %v", ns, err)
		}

		key := cv.ValueFrom.ConfigMapKeyRef.Key
		ret, ok := cm.Data[key]
		if !ok {
			return "", fmt.Errorf("key %s not found in configmap %s", key, ns)
		}
		return ret, nil
	} else if cv.ValueFrom.SecretKeyRef != nil {
		ns := types.NamespacedName{
			Namespace: cv.ValueFrom.SecretKeyRef.Namespace,
			Name:      cv.ValueFrom.SecretKeyRef.Name,
		}
		var sc v1.Secret
		if err := r.Client.Get(context.Background(), ns, &sc); err != nil {
			return "", fmt.Errorf("error getting secret %s: %v", ns, err)
		}

		key := cv.ValueFrom.SecretKeyRef.Key
		ret, ok := sc.Data[key]
		if !ok {
			return "", fmt.Errorf("key %s not found in secret %s", key, ns)
		}
		return string(ret), nil
	}

	return "", fmt.Errorf("one of secretKeyRef and configMapKeyRef must be set")
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
