package controllers

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	wasmxdsv1alpha1 "github.com/tetratelabs/wasmxds/api/v1alpha1"
	"github.com/tetratelabs/wasmxds/wasmxds"
)

var (
	k8sClient          client.Client
	mgr                ctrl.Manager
	namespace          = "wasmxds-test-wasmextension-controller"
	useExistingCluster = true
)

func TestMain(m *testing.M) {
	ctrl.SetLogger(zap.New())
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:  []string{"../manifests/crd/bases"},
		UseExistingCluster: &useExistingCluster,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		log.Fatal(err)
	}
	err = wasmxdsv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Fatal(err)
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Fatal(err)
	}

	mgr, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		Namespace:          namespace,
		MetricsBindAddress: ":0",
	})

	if err := k8sClient.Create(context.Background(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}); err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := k8sClient.Delete(context.Background(), &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}); err != nil {
			log.Fatal(err)
		}
		if err := testEnv.Stop(); err != nil {
			log.Fatal(err)
		}
	}()
	m.Run()
}

type lastHandled struct {
	extension              *wasmxdsv1alpha1.WasmExtension
	pluginConfig, vmConfig string
	updated, deleted       bool
}

func (m *lastHandled) Update(extension *wasmxdsv1alpha1.WasmExtension, pluginConfig, vmConfig string) (r ctrl.Result, e error) {
	m.extension = extension
	m.updated = true
	m.pluginConfig = pluginConfig
	m.vmConfig = vmConfig
	return
}

func (m *lastHandled) Delete(extension *wasmxdsv1alpha1.WasmExtension) {
	m.extension = extension
	m.deleted = true
}

func (m *lastHandled) reset() {
	m.extension = nil
	m.deleted = false
	m.updated = false
}

func (m *lastHandled) wait() {
	for m.extension == nil {
		time.Sleep(time.Millisecond * 100)
	}
}

var _ wasmxds.EventHandler = &lastHandled{}

func TestWasmExtensionReconciler_Reconcile(t *testing.T) {
	r := &WasmExtensionReconciler{
		Client: k8sClient,
		Log:    ctrl.Log.WithName("controllers").WithName("WasmExtension"),
		Scheme: scheme.Scheme,
	}
	ctx := context.Background()
	handler := &lastHandled{}

	require.NoError(t, r.SetupWithManager(mgr))
	r.SetEventHandler(handler)

	go func() {
		require.NoError(t, mgr.Start(ctrl.SetupSignalHandler()))
	}()

	name := "created-updated"
	t.Run(name, func(t *testing.T) {
		namespaced := types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}
		crd := &wasmxdsv1alpha1.WasmExtension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: wasmxdsv1alpha1.WasmExtensionSpec{
				Image: wasmxdsv1alpha1.WasmExtensionSpecImage{},
			},
		}

		require.NoError(t, r.Client.Create(ctx, crd))
		handler.wait()
		assert.True(t, handler.updated)
		assert.Equal(t, name, handler.extension.Name)
		handler.reset()

		require.NoError(t, r.Get(ctx, namespaced, crd))
		expVMConfig := "this is vm configuration"
		crd.Spec.VMConfiguration = &wasmxdsv1alpha1.WasmExtensionConfigValue{
			Value: &expVMConfig,
		}
		expPluginConfig := "this is plugin configuration"
		crd.Spec.PluginConfiguration = &wasmxdsv1alpha1.WasmExtensionConfigValue{
			Value: &expPluginConfig,
		}
		require.NoError(t, r.Client.Update(ctx, crd))
		handler.wait()
		assert.True(t, handler.updated)
		assert.Equal(t, name, handler.extension.Name)
		if ptr := handler.extension.Spec.VMConfiguration; assert.NotNil(t, ptr) && assert.NotNil(t, ptr.Value) {
			assert.Equal(t, expVMConfig, *ptr.Value)
		}
		if ptr := handler.extension.Spec.PluginConfiguration; assert.NotNil(t, ptr) && assert.NotNil(t, ptr.Value) {
			assert.Equal(t, expPluginConfig, *ptr.Value)
		}
		handler.reset()

		require.NoError(t, r.Get(ctx, namespaced, crd))
		require.NoError(t, r.Client.Delete(ctx, crd))
		handler.wait()
		assert.True(t, handler.deleted)
		assert.Equal(t, name, handler.extension.Name)
		handler.reset()

		// verify the object not exist
		require.True(t, errors.IsNotFound(r.Get(ctx, namespaced, crd)))
	})

	name = "created-deleted"
	t.Run(name, func(t *testing.T) {
		namespaced := types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}
		crd := &wasmxdsv1alpha1.WasmExtension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: wasmxdsv1alpha1.WasmExtensionSpec{
				Image: wasmxdsv1alpha1.WasmExtensionSpecImage{},
			},
		}

		require.NoError(t, r.Client.Create(ctx, crd))
		handler.wait()
		assert.True(t, handler.updated)
		assert.Equal(t, name, handler.extension.Name)
		handler.reset()

		require.NoError(t, r.Get(ctx, namespaced, crd))
		require.NoError(t, r.Client.Delete(ctx, crd))
		handler.wait()
		assert.True(t, handler.deleted)
		assert.Equal(t, name, handler.extension.Name)
		handler.reset()

		// verify the object not exist
		require.True(t, errors.IsNotFound(r.Get(ctx, namespaced, crd)))
	})
}

func TestWasmExtensionReconciler_resolveConfig(t *testing.T) {
	r := &WasmExtensionReconciler{
		Client: k8sClient,
		Log:    ctrl.Log.WithName("controllers").WithName("WasmExtension"),
		Scheme: scheme.Scheme,
	}

	t.Run("raw", func(t *testing.T) {
		exp := "exp"
		actual, err := r.resolveConfig(&wasmxdsv1alpha1.WasmExtensionConfigValue{
			Value: &exp,
		})
		require.NoError(t, err)
		assert.Equal(t, exp, actual)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := r.resolveConfig(&wasmxdsv1alpha1.WasmExtensionConfigValue{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "one of valueFrom and value must be set")

		_, err = r.resolveConfig(&wasmxdsv1alpha1.WasmExtensionConfigValue{
			ValueFrom: &wasmxdsv1alpha1.WasmExtensionConfigValueRef{}},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "one of secretKeyRef and configMapKeyRef must be set")
	})

	t.Run("configMapKeyRef", func(t *testing.T) {
		key := "key"
		value := "value"
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test1",
				Namespace: "default",
			},
			Data: map[string]string{key: value},
		}

		defer func() {
			require.NoError(t, r.Client.Delete(context.Background(), cm))
		}()

		require.NoError(t, r.Client.Create(context.Background(), cm))

		attr := &wasmxdsv1alpha1.WasmExtensionConfigValueRefAttribute{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Key:       key,
		}
		actual, err := r.resolveConfig(&wasmxdsv1alpha1.WasmExtensionConfigValue{
			ValueFrom: &wasmxdsv1alpha1.WasmExtensionConfigValueRef{
				ConfigMapKeyRef: attr,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, value, actual)

		attr.Key = "non-exist"
		actual, err = r.resolveConfig(&wasmxdsv1alpha1.WasmExtensionConfigValue{
			ValueFrom: &wasmxdsv1alpha1.WasmExtensionConfigValueRef{
				ConfigMapKeyRef: attr,
			},
		})
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("secretKeyRef", func(t *testing.T) {
		key := "key"
		exp := []byte("hello world")
		sc := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test1",
				Namespace: "default",
			},
			Data: map[string][]byte{key: exp},
		}

		defer func() {
			require.NoError(t, r.Client.Delete(context.Background(), sc))
		}()

		require.NoError(t, r.Client.Create(context.Background(), sc))

		attr := &wasmxdsv1alpha1.WasmExtensionConfigValueRefAttribute{
			Name:      sc.Name,
			Namespace: sc.Namespace,
			Key:       key,
		}
		actual, err := r.resolveConfig(&wasmxdsv1alpha1.WasmExtensionConfigValue{
			ValueFrom: &wasmxdsv1alpha1.WasmExtensionConfigValueRef{
				SecretKeyRef: attr,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, string(exp), actual)

		attr.Key = "non-exist"
		actual, err = r.resolveConfig(&wasmxdsv1alpha1.WasmExtensionConfigValue{
			ValueFrom: &wasmxdsv1alpha1.WasmExtensionConfigValueRef{
				SecretKeyRef: attr,
			},
		})
		require.Error(t, err)
		t.Log(err)
	})
}

func TestWasmExtensionReconciler_resolveConfigs(t *testing.T) {
	r := &WasmExtensionReconciler{Log: ctrl.Log}

	pluginConfigValue := "plugin"
	vmConfigValue := "vm"
	crd := &wasmxdsv1alpha1.WasmExtension{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: wasmxdsv1alpha1.WasmExtensionSpec{
			PluginConfiguration: &wasmxdsv1alpha1.WasmExtensionConfigValue{
				Value: &pluginConfigValue,
			},
			VMConfiguration: &wasmxdsv1alpha1.WasmExtensionConfigValue{
				Value: &vmConfigValue,
			},
		},
	}

	pc, vc, err := r.resolveConfigs(crd)
	require.NoError(t, err)
	assert.Equal(t, pluginConfigValue, pc)
	assert.Equal(t, vmConfigValue, vc)

	crd.Spec.VMConfiguration = nil
	pc, vc, err = r.resolveConfigs(crd)
	require.NoError(t, err)
	assert.Equal(t, pluginConfigValue, pc)
	assert.Equal(t, "", vc)

	crd.Spec.PluginConfiguration = nil
	pc, vc, err = r.resolveConfigs(crd)
	require.NoError(t, err)
	assert.Equal(t, "", pc)
	assert.Equal(t, "", vc)

	crd.Spec.VMConfiguration = &wasmxdsv1alpha1.WasmExtensionConfigValue{Value: &vmConfigValue}
	pc, vc, err = r.resolveConfigs(crd)
	require.NoError(t, err)
	assert.Equal(t, "", pc)
	assert.Equal(t, vmConfigValue, vc)
}
