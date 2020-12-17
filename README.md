# Wasmxds - Wasm Extension Discovery Service

[![Build](https://github.com/tetratelabs/wasmxds/workflows/build-test/badge.svg)](https://github.com/tetratelabs/wasmxds/actions)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)


__Wasmxds (Wasm Extension Discovery Service)__ is an implementation of Extension Configuration Discovery Service (ECDS) as a Kubernetes operator,
 which enables users to dynamically and flexibly configure [Proxy-Wasm] extensions in [Envoy] fleets.

Wasmxds is able to fetch Wasm binaries from variety of places such as [Amazon S3], http servers, OCI compliant registries including [Amazon ECR] and [WebAssembly Hub], and more.

![architecture](https://raw.githubusercontent.com/tetratelabs/wasmxds/main/docs/architecture.jpeg?token=ADHDJ6OSOCAGNHCSVUUPZGS74P2J2)

## Installation

```
kubectl apply -f https://raw.githubusercontent.com/tetratelabs/wasmxds/main/manifests/wasmxds.yaml
```

After the installation and creation of WasmExtension custom resources, configure your Envoy fleets and tell them to get Wasm extensions from Wasmxds' k8s service. 
See examples/envoy.yaml for details.

## Custom Resource Definition explained

Wasmxds has one CRD to fetch and prepare your Wasm Extensions:

```yaml
apiVersion: wasmxds.tetrate.io/v1alpha1
kind: WasmExtension
metadata:
  # "${namespace}/${name}" (i.e. "default/sample-filter") to be used as an identifier in Envoy configuration.
  # See the extension's name field in examples/envoy.yaml
  name: sample-filter
  namespace: default

spec:
  # Please refer to https://github.com/envoyproxy/envoy/blob/master/api/envoy/extensions/wasm/v3/wasm.proto
  # for the following values
  runtime: v8 # (required)
  vm_id: vm_id_foo # (required)
  root_id: root_id_foo # (required)
  # you can use your k8s configmap/secret or inline string as providing plugin/vm configurations
  plugin_configuration:
    # value: "this_is_plugin_config"
    valueFrom:
      secretKeyRef:
        name: my-secret
        namespace: my-secret-space
        key: my-config-key
      # configMapKeyRef:
      #   name: my-configmap
      #   namespace: my-config-space
      #    key: my-config-key
  vm_configuration:
    # value: "this_is_vm_config"
    valueFrom:
      # secretKeyRef:
      #   name: my-secret
      #   namespace: my-secret-space
      #   key: my-config-key
      configMapKeyRef:
        name: my-configmap
        namespace: my-config-space
          key: my-config-key

  image:
    # Specify the protocol to use for fetching wasm binaries. (required).
    # See api/v1alpha/wasmextension_types.go to check the available protocol.
    protocol: local_fs

    # The resource identifier which points to your wasm binary. (required).
    uri: filter.wasm

    # This field is the expected sha256 value of your wasm binary, and is
    # optional but should be set before shipping to production
    # sha256: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx


    ### The followings are the examples for the other protocols ###

    # uri: webassemblyhub.io/mathetake/example:v0.1
    # protocol: oci

    # uri: 123456789012.dkr.ecr.us-west-1.amazonaws.com/wasmxds-test:latest
    # protocol: oci

    # uri: my-s3-bucket/path/to/filter.wasm
    # protocol: s3

    # uri: foo.com/assets/filter.wasm
    # protocol: http

    # uri: bar.com/assets/filter.wasm
    # protocol: https
```

## OCI image packaging

You can package your Wasm binary to an OCI image compliant image by using tools like [wasm-to-oci], and push them to the OCI compliant registries,
such as [Amazon ECR]. This is done by making use of the [OCI Artifact proposal] which may not be supported by 
some container registries including [Docker Hub]. Currently, the following artifact types are supported by Wasmxds:

- application/vnd.module.wasm.content.layer.v1+wasm
- application/vnd.wasm.content.layer.v1+wasm

## Limitations

Currently, Envoy only supports the filter configuration discovery for Http filter chains.

## Development

### code generation

```
make codegen
```

### tests

There are four types of tests:

| test \ require | k8s cluster | docker-compose up |
|:-------------:|:-------------:|:-------------:|
| make gotest | NO |   YES |
| make e2e.controllers |  YES | NO |
| make e2e.providers |  YES | YES |
| make e2e.k8s |  YES | NO |

The test requiring k8s actually operate on the current kubectl context. 
So before you run tests, please make sure you have:
- a [kind] cluster in your local machine to avoid destroying any remote k8s cluster
- your kubectl context switched to that cluster
- docker/docker-compose installed

[Amazon S3]: https://aws.amazon.com/s3
[Amazon ECR]: https://aws.amazon.com/ecr/
[Proxy-Wasm]: https://github.com/proxy-wasm
[Envoy]: https://github.com/envoyproxy/envoy
[WebAssembly Hub]: https://webassemblyhub.io/
[wasm-to-oci]: https://github.com/engineerd/wasm-to-oci
[OCI Artifact proposal]: https://github.com/opencontainers/artifacts
[Docker Hub]: https://hub.docker.com/
[kind]: https://kind.sigs.k8s.io/
