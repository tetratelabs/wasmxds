package wasmxds

import (
	"context"
	"errors"
	"testing"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	wasmxdsv1alpha1 "github.com/tetratelabs/wasmxds/api/v1alpha1"
	"github.com/tetratelabs/wasmxds/imageprovider"
)

func strPtr(s string) *string {
	return &s
}

func TestServer_Update(t *testing.T) {
	s := Server{
		imageCache: map[string][]byte{"url": {1, 2, 3}},
		cache:      cache.NewLinearCache(apiType),
		logger:     zap.New(),
	}

	ext := &wasmxdsv1alpha1.WasmExtension{}
	ext.Spec.Image = wasmxdsv1alpha1.WasmExtensionSpecImage{
		URI: "url", Sha256: strPtr("039058c6f2c0cb492c533b0a4d14ef77cc0f78abccced5287d84a1a2011cfb81"),
	}
	_, err := s.Update(ext, "", "")
	assert.NoError(t, err)

	ext.Spec.Image.Sha256 = strPtr("not match")
	_, err = s.Update(ext, "", "")
	assert.Error(t, err)

	ext.Spec.Image.Sha256 = nil
	_, err = s.Update(ext, "", "")
	assert.NoError(t, err)
}

func TestServer_Delete(t *testing.T) {
	key := "cached"
	s := Server{
		imageCache: map[string][]byte{key: {}},
		cache:      cache.NewLinearCache(apiType),
		logger:     zap.New(),
	}

	ext := &wasmxdsv1alpha1.WasmExtension{}
	ext.Spec.Image.URI = key
	s.Delete(ext)
	_, ok := s.imageCache[key]
	assert.False(t, ok)

}

type fakeProvider struct {
	binaries    map[string][]byte
	providerKey string
}

var ErrFakeNotFound = errors.New("not found by fake")

func (f *fakeProvider) Fetch(_ context.Context, uri string) ([]byte, error) {
	b, ok := f.binaries[uri]
	if ok {
		return b, nil
	}
	return nil, ErrFakeNotFound
}

func (f *fakeProvider) ProviderKey() string {
	return f.providerKey
}

func TestServer_fetchImage(t *testing.T) {
	foundURI := "webassemblyhub.com/tetrate.io/sample-filter:v2"
	providers := []imageprovider.WasmImageProvider{
		&fakeProvider{binaries: map[string][]byte{}, providerKey: "local_fs"},
		&fakeProvider{binaries: map[string][]byte{
			foundURI: {1, 2, 3},
		}, providerKey: "oci||webassemblyhub.com"},
	}

	s := Server{imageCache: map[string][]byte{}, imageProviders: map[string]imageprovider.WasmImageProvider{}}
	for _, p := range providers {
		s.imageProviders[p.ProviderKey()] = p
	}

	for _, c := range []struct{ uri, protocol string }{
		{uri: "aaa.wasm", protocol: "unsupported_protocol"},
		{uri: "nonexist.com/tetrate.io/sample-filter:v1", protocol: "oci"}, // provider not registered
	} {
		_, err := s.fetchImage(&wasmxdsv1alpha1.WasmExtensionSpecImage{
			URI: c.uri, Protocol: c.protocol,
		})
		assert.Error(t, err)
		t.Log(err.Error())
	}

	for _, c := range []struct{ uri, protocol string }{
		{uri: "aaa.wasm", protocol: "local_fs"},
		{uri: "webassemblyhub.com/tetrate.io/sample-filter:v1", protocol: "oci"},
	} {
		_, err := s.fetchImage(&wasmxdsv1alpha1.WasmExtensionSpecImage{
			URI: c.uri, Protocol: c.protocol,
		})
		assert.True(t, errors.Is(err, ErrFakeNotFound), err.Error())
	}

	actual, err := s.fetchImage(&wasmxdsv1alpha1.WasmExtensionSpecImage{
		URI: foundURI, Protocol: "oci",
	})
	assert.NoError(t, err)
	assert.Equal(t, []byte{1, 2, 3}, actual)
	assert.Equal(t, []byte{1, 2, 3}, s.imageCache[foundURI])
}
