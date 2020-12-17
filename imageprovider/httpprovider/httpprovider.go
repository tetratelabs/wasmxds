package httpprovider

import (
	"context"
	"fmt"
	"net/http"

	wasmxdsv1alpha1 "github.com/tetratelabs/wasmxds/api/v1alpha1"
)

type HttpProvider struct {
	client http.Client
}

func NewHttpProvider() *HttpProvider {
	return &HttpProvider{client: http.Client{}}
}

func (h HttpProvider) Fetch(_ context.Context, uri string) ([]byte, error) {
	return get(h.client, fmt.Sprintf("http://%s", uri))
}

func (h HttpProvider) ProviderKey() string {
	return wasmxdsv1alpha1.ProtocolHttp
}
