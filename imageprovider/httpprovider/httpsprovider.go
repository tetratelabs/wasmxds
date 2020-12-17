package httpprovider

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	wasmxdsv1alpha1 "github.com/tetratelabs/wasmxds/api/v1alpha1"
)

type HttpsProvider struct {
	client http.Client
}

func NewHttpsProvider(insecure bool) *HttpsProvider {
	client := http.Client{}
	if insecure {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	return &HttpsProvider{client: client}
}

func (h HttpsProvider) Fetch(_ context.Context, uri string) ([]byte, error) {
	return get(h.client, fmt.Sprintf("https://%s", uri))
}

func (h HttpsProvider) ProviderKey() string {
	return wasmxdsv1alpha1.ProtocolHttps
}
