package k8s

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestE2e(t *testing.T) {
	if image := os.Getenv("WASMXDS_IMAGE"); image != "" {
		patch := fmt.Sprintf(
			`{"spec":{"template":{"spec":{"containers":[{"name":"manager","image":"%s"}]}}}}`,
			image,
		)

		cmd := exec.Command("kubectl", "patch", "deployment",
			"-n", "wasmxds-system", "wasmxds-controller-manager", "-p", patch)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		require.NoError(t, cmd.Run())
	}
	time.Sleep(10 * time.Second)

	name := "extension-test"

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = bytes.NewReader([]byte(configManifests))
	require.NoError(t, cmd.Run())
	t.Log("sleep for a few seconds...")
	time.Sleep(3 * time.Second)

	manifest := fmt.Sprintf(extensionManifestTemplate, name)
	cmd = exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = bytes.NewReader([]byte(manifest))
	require.NoError(t, cmd.Run())
	t.Log("sleep for a few seconds...")
	time.Sleep(10 * time.Second)

	defer func() {
		cmd = exec.Command("kubectl", "delete", "-f", "-")
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Stdin = bytes.NewReader([]byte(configManifests))
		require.NoError(t, cmd.Run())
		cmd = exec.Command("kubectl", "delete", "-f", "-")
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Stdin = bytes.NewReader([]byte(manifest))
		require.NoError(t, cmd.Run())
	}()

	cmd = exec.Command("kubectl", "-n", "wasmxds-system", "logs", "-l", "tetrate.io=wasmxds")
	buf := new(bytes.Buffer)
	cmd.Stderr = os.Stderr
	cmd.Stdout = buf
	require.NoError(t, cmd.Start())

	time.Sleep(5 * time.Second)
	require.NoError(t, cmd.Process.Kill())
	actual := buf.String()
	for _, exp := range []string{
		fmt.Sprintf(`"msg":"reconciliation successfully finished","name":"default/%s"`, name),
		fmt.Sprintf(`"msg":"plugin configuration resolved","name":"default/%s"`, name),
		fmt.Sprintf(`"msg":"vm configuration resolved","name":"default/%s"`, name),
	} {
		require.Contains(t, actual, exp, actual)
	}
	fmt.Println(actual)
}

const extensionManifestTemplate = `
apiVersion: wasmxds.tetrate.io/v1alpha1
kind: WasmExtension
metadata:
  name: %s
spec:
  runtime: v8
  vm_id: vm_id_foo
  root_id: root_id_foo
  image:
    protocol: https
    uri: mathetake.github.io/assets/wasm/filter.https.wasm
  vm_configuration:
    valueFrom:
      secretKeyRef:
        name: sample
        namespace: default
        key: key
  plugin_configuration:
    valueFrom:
      configMapKeyRef:
        name: sample
        namespace: default
        key: key
`

const configManifests = `
apiVersion: v1
kind: Secret
metadata:
  name: sample
type: Opaque
data:
  key: aGVsbG8gd29ybGQ=
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: sample
data:
  key: hello
`
