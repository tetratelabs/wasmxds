package httpprovider

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bmizerany/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpsProvider(t *testing.T) {
	exp := []byte{1, 2, 3}
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(exp)
	}))

	defer ts.Close()

	p := HttpsProvider{client: http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}}

	actual, err := p.Fetch(nil, strings.TrimPrefix(ts.URL, "https://"))
	require.NoError(t, err)
	assert.Equal(t, exp, actual)
}
