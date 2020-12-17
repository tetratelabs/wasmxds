package localfs

import (
	"context"
	"io/ioutil"

	wasmxdsv1alpha1 "github.com/tetratelabs/wasmxds/api/v1alpha1"
)

type LocalFilesystem struct{}

func (l LocalFilesystem) Fetch(_ context.Context, uri string) ([]byte, error) {
	return ioutil.ReadFile(uri)
}

func (l LocalFilesystem) ProviderKey() string {
	return wasmxdsv1alpha1.ProtocolLocalFileSystem
}
