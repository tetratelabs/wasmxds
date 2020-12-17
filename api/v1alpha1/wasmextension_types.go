// Copyright 2020 Tetrate
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"fmt"

	"github.com/containerd/containerd/reference"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WasmExtensionSpec defines the desired state of WasmExtension
type WasmExtensionSpec struct {
	Image               WasmExtensionSpecImage    `json:"image"`
	VMID                string                    `json:"vm_id"`
	RootID              string                    `json:"root_id"`
	VMConfiguration     *WasmExtensionConfigValue `json:"vm_configuration,omitempty"`
	PluginConfiguration *WasmExtensionConfigValue `json:"plugin_configuration,omitempty"`
	Runtime             string                    `json:"runtime"`
}

type WasmExtensionSpecImage struct {
	URI      string  `json:"uri"`
	Protocol string  `json:"protocol"`
	Sha256   *string `json:"sha256,omitempty"`
}

type WasmExtensionConfigValue struct {
	Value     *string                      `json:"value,omitempty"`
	ValueFrom *WasmExtensionConfigValueRef `json:"valueFrom,omitempty"`
}

type WasmExtensionConfigValueRef struct {
	SecretKeyRef    *WasmExtensionConfigValueRefAttribute `json:"secretKeyRef,omitempty"`
	ConfigMapKeyRef *WasmExtensionConfigValueRefAttribute `json:"configMapKeyRef,omitempty"`
}

type WasmExtensionConfigValueRefAttribute struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
}

// WasmExtensionStatus defines the observed state of WasmExtension
type WasmExtensionStatus struct{}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// WasmExtension is the Schema for the wasmextensions API
type WasmExtension struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WasmExtensionSpec   `json:"spec,omitempty"`
	Status WasmExtensionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WasmExtensionList contains a list of WasmExtension
type WasmExtensionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WasmExtension `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WasmExtension{}, &WasmExtensionList{})
}

// ProviderKey corresponds to `WasmImageProvider.ProviderKey()`
func (in *WasmExtensionSpecImage) ProviderKey() (string, error) {
	protocol := in.Protocol
	if protocol == "" {
		protocol = ProtocolOCIImageRegistry
	}

	switch protocol {
	case ProtocolOCIImageRegistry:
		s, err := reference.Parse(in.URI)
		if err != nil {
			return "", fmt.Errorf("failed to parse URI as OCI ref %s: %w", in.URI, err)
		}
		return fmt.Sprintf("%s||%s", protocol, s.Hostname()), nil
	case ProtocolLocalFileSystem, ProtocolS3, ProtocolHttp, ProtocolHttps:
		return protocol, nil
	default:
		return "", fmt.Errorf("unsupported protoco: %s", protocol)
	}
}

func (in *WasmExtensionSpecImage) ID() string {
	return fmt.Sprintf("%s://%s", in.Protocol, in.URI)
}

func (in *WasmExtension) Namespaced() string {
	return fmt.Sprintf("%s/%s", in.Namespace, in.Name)
}

const (
	ProtocolOCIImageRegistry = "oci"
	ProtocolLocalFileSystem  = "local_fs"
	ProtocolS3               = "s3"
	ProtocolHttp             = "http"
	ProtocolHttps            = "https"
	// TODO: add more protocol: e.g. gcs, ...
)
