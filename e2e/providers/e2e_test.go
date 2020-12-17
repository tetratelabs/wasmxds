package providers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wasmxdsv1alpha1 "github.com/tetratelabs/wasmxds/api/v1alpha1"
	"github.com/tetratelabs/wasmxds/imageprovider/ociregistory"
)

var (
	namespace        = "wasmxds-test-e2e"
	localOCIRef      = "localhost:5000/wasmxds/wasm-filter:v1"
	amazonECROCIRef  = "%s/wasmxds-ci-test:e2e"
	amazonS3URI      = "wasmxds-test-backet/path/to/filter.wasm"
	httpUri          = "%s/filter.wasm"
	httpsUri         = "%s/filter.wasm"
	protocolToSha256 = map[string]string{}
)

func TestMain(m *testing.M) {
	err := os.Chdir("../..")
	if err != nil {
		log.Fatal(err)
	}
	m.Run()
}

func runHttpServer(t *testing.T) func() {
	contents, err := ioutil.ReadFile("e2e/providers/testdata/filter.http.wasm")
	require.NoError(t, err)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(contents)
	}))

	httpUri = fmt.Sprintf(httpUri, strings.TrimPrefix(ts.URL, "http://"))
	raw := sha256.Sum256(contents)
	protocolToSha256[wasmxdsv1alpha1.ProtocolHttp] = hex.EncodeToString(raw[:])
	return ts.Close
}

func runHttpsServer(t *testing.T) func() {
	contents, err := ioutil.ReadFile("e2e/providers/testdata/filter.https.wasm")
	require.NoError(t, err)
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(contents)
	}))

	httpsUri = fmt.Sprintf(httpsUri, strings.TrimPrefix(ts.URL, "https://"))
	raw := sha256.Sum256(contents)
	protocolToSha256[wasmxdsv1alpha1.ProtocolHttps] = hex.EncodeToString(raw[:])
	return ts.Close
}

func putToS3(t *testing.T) {
	sess, err := session.NewSession(aws.NewConfig().
		WithRegion("us-west-1").
		WithCredentials(credentials.NewStaticCredentials("dummy", "dummy", "")).
		WithS3ForcePathStyle(true).
		WithEndpoint("http://localhost:4566"))
	require.NoError(t, err)

	contents, err := ioutil.ReadFile("e2e/providers/testdata/filter.s3.wasm")
	require.NoError(t, err)

	u := strings.SplitN(amazonS3URI, "/", 2)
	{
		client := s3.New(sess)
		_, _ = client.CreateBucket(&s3.CreateBucketInput{
			Bucket: aws.String(u[0]),
		})
	}
	{
		client := s3manager.NewUploader(sess)
		in := bytes.NewReader(contents)
		_, err = client.Upload(&s3manager.UploadInput{
			Body:   in,
			Bucket: aws.String(u[0]),
			Key:    aws.String(u[1]),
		})
		require.NoError(t, err)
	}

	raw := sha256.Sum256(contents)
	protocolToSha256[wasmxdsv1alpha1.ProtocolS3] = hex.EncodeToString(raw[:])
}

func pushOCIImage(t *testing.T) {
	contents, err := ioutil.ReadFile("e2e/providers/testdata/filter.oci.wasm")
	require.NoError(t, err)

	raw := sha256.Sum256(contents)
	protocolToSha256[wasmxdsv1alpha1.ProtocolOCIImageRegistry] = hex.EncodeToString(raw[:])

	localRegistry := ociregistory.NewLocalRegistry("", "", "5000")
	err = localRegistry.Push(contents, localOCIRef)
	require.NoError(t, err)

	if testAgainstAmazonECR {
		sess, err := session.NewSession()
		require.NoError(t, err)
		aes, err := ociregistory.NewAmazonECR(sess)
		require.NoError(t, err)

		var ae *ociregistory.AmazonECR
		for _, a := range aes {
			if strings.Contains(a.ProviderKey(), "us-west-1") {
				ae = a
				break
			}
		}
		if ae == nil {
			t.Fatal("unable to fine image provider for Amazon ECR")
		}
		amazonECROCIRef = fmt.Sprintf(amazonECROCIRef, ae.Host())
		err = ae.Push(contents, amazonECROCIRef)
		require.NoError(t, err)
	}
}

func TestRunE2E(t *testing.T) {
	t.Log("pushing to OCI registry...")
	pushOCIImage(t)

	t.Log("pushing to local Amazon s3 ...")
	putToS3(t)

	t.Log("running http server which serves a Wasm binary...")
	httpDone := runHttpServer(t)
	defer httpDone()

	t.Log("running https server serves a Wasm binary ...")
	httpsDone := runHttpsServer(t)
	defer httpsDone()

	contents, err := ioutil.ReadFile("e2e/providers/testdata/filter.local_fs.wasm")
	require.NoError(t, err)
	raw := sha256.Sum256(contents)
	protocolToSha256[wasmxdsv1alpha1.ProtocolLocalFileSystem] = hex.EncodeToString(raw[:])

	t.Log("run controller")
	args := []string{"-n", namespace, "-s3-local=true", "-insecure-https=true"}
	if testAgainstAmazonECR {
		args = append(args, "-ecr=true")
	}
	managerCmd := exec.Command("bin/manager", args...)
	managerCmd.Stdout = os.Stdout
	managerCmd.Stderr = os.Stderr
	require.NoError(t, managerCmd.Start())

	defer func() {
		if err := exec.Command("kubectl", "delete", "ns", namespace).Run(); err != nil {
			log.Fatal(err)
		}
		time.Sleep(5 * time.Second) // wait for garbage collection
		require.NoError(t, managerCmd.Process.Kill())
	}()

	cmd := exec.Command("kubectl", "create", "ns", namespace)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	t.Log("sleep for a few seconds...")
	time.Sleep(10 * time.Second) // maybe flaky

	type tc struct {
		uri, protocol,
		vmConfig, pluginConfig string
	}

	cases := []tc{
		{
			uri:          httpUri,
			protocol:     wasmxdsv1alpha1.ProtocolHttp,
			vmConfig:     "v11",
			pluginConfig: "p12",
		},
		{
			uri:          httpsUri,
			protocol:     wasmxdsv1alpha1.ProtocolHttps,
			vmConfig:     "v31",
			pluginConfig: "p14t22",
		},
		{
			uri:          "e2e/providers/testdata/filter.local_fs.wasm",
			protocol:     wasmxdsv1alpha1.ProtocolLocalFileSystem,
			vmConfig:     "v1",
			pluginConfig: "p1",
		},
		{
			uri:          localOCIRef,
			protocol:     wasmxdsv1alpha1.ProtocolOCIImageRegistry,
			vmConfig:     "v1",
			pluginConfig: "p2",
		},
		{
			uri:          amazonS3URI,
			protocol:     wasmxdsv1alpha1.ProtocolS3,
			vmConfig:     "v2",
			pluginConfig: "p3",
		},
	}

	if testAgainstAmazonECR {
		cases = append(cases,
			tc{uri: amazonECROCIRef, protocol: wasmxdsv1alpha1.ProtocolOCIImageRegistry,
				vmConfig: "v3", pluginConfig: "p3"},
		)
	}

	for _, c := range cases {
		manifest := getManifest(c.uri, c.protocol, c.vmConfig, c.pluginConfig)
		cmd := exec.Command("kubectl", "apply",
			"-f", "-")
		t.Log("kubectl apply: ", manifest)
		cmd.Stdin = bytes.NewBuffer([]byte(manifest))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		require.NoError(t, cmd.Run())

		t.Log("wait for reconciliation...")
		time.Sleep(10 * time.Second) // maybe flaky
		req, err := http.NewRequest("GET", "http://localhost:18000", nil)
		require.NoError(t, err)

		var succeed bool
		var r *http.Response
		for i := 0; i < 10; i++ {
			r, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			succeed = r.StatusCode == http.StatusOK
			if succeed {
				break
			}
			t.Log("envoy unhealthy...")
			time.Sleep(time.Second)
			continue
		}

		require.True(t, succeed, "Envoy unhealthy")
		checkHeader := func(expKey, expVal string) {
			actual := r.Header.Get(expKey)
			assert.Equal(t, expVal, actual)
		}
		checkHeader("protocol", c.protocol)
		checkHeader("vm-configuration", c.vmConfig)
		checkHeader("plugin-configuration", c.pluginConfig)
		t.Log(r.Header)
		r.Body.Close()
	}

	deleteCmd := exec.Command("kubectl", "delete", "wasmextensions.wasmxds.tetrate.io",
		"-n", namespace, "sample-filter")
	deleteCmd.Stdout = os.Stdout
	deleteCmd.Stderr = os.Stderr
	if err := deleteCmd.Run(); err != nil {
		log.Fatal(err)
	}
}

func getManifest(uri, protocol, vmConfig, pluginConfig string) string {
	const manifestTemplate = `
apiVersion: wasmxds.tetrate.io/v1alpha1
kind: WasmExtension
metadata:
  name: sample-filter
  namespace: %s
spec:
  image:
    uri: %s
    protocol: %s
    sha256: %s
  runtime: v8
  vm_id: "vm_id"
  root_id: "root_id"
  vm_configuration:
    value: %s
  plugin_configuration:
    value: %s`
	return fmt.Sprintf(manifestTemplate, namespace, uri, protocol, protocolToSha256[protocol], vmConfig, pluginConfig)
}
