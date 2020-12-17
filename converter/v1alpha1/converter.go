// Copyright Istio Authors
// Copyright Tetrate
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Some of the code here is extracted from
// https://github.com/istio/proxy/blob/85a0d22426f71369e6db75558adc2c7ae50bda05/tools/extensionserver/convert.go

package v1alpha1

import (
	"fmt"
	"strings"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	wasm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"
	"go.uber.org/zap"

	wasmxdsv1alpha1 "github.com/tetratelabs/wasmxds/api/v1alpha1"
)

func Convert(ext *wasmxdsv1alpha1.WasmExtension, binary []byte, pluginConfig, vmConfig string) (*core.TypedExtensionConfig, error) {
	pc, err := ptypes.MarshalAny(&wrappers.StringValue{Value: pluginConfig})
	if err != nil {
		return nil, fmt.Errorf("marshal plugin configuration failed: %w", err)
	}

	vc, err := ptypes.MarshalAny(&wrappers.StringValue{Value: vmConfig})
	if err != nil {
		return nil, fmt.Errorf("marshal vm configuration failed: %w", err)
	}

	// detect the runtime
	runtime := "envoy.wasm.runtime.v8"
	switch strings.ToLower(ext.Spec.Runtime) {
	case "v8", "":
		break
	case "wavm":
		runtime = "envoy.wasm.runtime.wavm"
	case "wasmtime":
		runtime = "envoy.wasm.runtime.wasmtime"
	default:
		zap.S().Errorf("unknown runtime %q. fall back to v8", ext.Spec.Runtime)
	}

	// create plugin config
	plugin := &wasm.Wasm{
		Config: &v3.PluginConfig{
			RootId: ext.Spec.RootID,
			VmConfig: &v3.PluginConfig_InlineVmConfig{
				InlineVmConfig: &v3.VmConfig{
					Configuration: vc,
					VmId:          ext.Spec.VMID,
					Runtime:       runtime,
					Code: &core.AsyncDataSource{
						Specifier: &core.AsyncDataSource_Local{
							Local: &core.DataSource{
								Specifier: &core.DataSource_InlineBytes{
									InlineBytes: binary,
								},
							},
						},
					},
					AllowPrecompiled: true,
				},
			},
			Configuration: pc,
		},
	}

	typed, err := ptypes.MarshalAny(plugin)
	if err != nil {
		return nil, err
	}
	return &core.TypedExtensionConfig{
		Name:        ext.Namespaced(),
		TypedConfig: typed,
	}, nil
}
