package httpprovider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bmizerany/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpProvider(t *testing.T) {
	exp := []byte{1, 2}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(exp)
	}))

	defer ts.Close()

	p := HttpProvider{}
	fmt.Println(ts.URL)
	actual, err := p.Fetch(nil, strings.TrimPrefix(ts.URL, "http://"))
	require.NoError(t, err)
	assert.Equal(t, exp, actual)
}
