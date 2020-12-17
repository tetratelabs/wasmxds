package imageprovider

import (
	"context"

	"github.com/tetratelabs/wasmxds/imageprovider/httpprovider"
	"github.com/tetratelabs/wasmxds/imageprovider/localfs"
	"github.com/tetratelabs/wasmxds/imageprovider/ociregistory"
	"github.com/tetratelabs/wasmxds/imageprovider/s3provider"
)

type WasmImageProvider interface {
	Fetch(ctx context.Context, uri string) ([]byte, error)
	ProviderKey() string
}

var (
	_ WasmImageProvider = &ociregistory.AmazonECR{}
	_ WasmImageProvider = ociregistory.WebAssemblyHub{}
	_ WasmImageProvider = ociregistory.LocalRegistry{}
	_ WasmImageProvider = localfs.LocalFilesystem{}
	_ WasmImageProvider = &s3provider.AmazonS3{}
	_ WasmImageProvider = &httpprovider.HttpProvider{}
	_ WasmImageProvider = &httpprovider.HttpsProvider{}
)
