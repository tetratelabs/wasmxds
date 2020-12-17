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
// https://github.com/istio/proxy/blob/85a0d22426f71369e6db75558adc2c7ae50bda05/tools/extensionserver/server.go

package wasmxds

import (
	"context"
	"errors"

	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	extensionservice "github.com/envoyproxy/go-control-plane/envoy/service/extension/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/tetratelabs/wasmxds/imageprovider"
)

const (
	apiType = "type.googleapis.com/envoy.config.core.v3.TypedExtensionConfig"
)

type Server struct {
	server.Server
	server.CallbackFuncs
	logger logr.Logger

	cache          *cache.LinearCache
	imageProviders map[string]imageprovider.WasmImageProvider
	imageCache     map[string][]byte
}

func NewServer(ctx context.Context, providers ...imageprovider.WasmImageProvider) (*Server, error) {
	if len(providers) == 0 {
		return nil, errors.New("at least one image providers must be given")
	}

	svr := &Server{
		imageProviders: make(map[string]imageprovider.WasmImageProvider, len(providers)),
		imageCache:     map[string][]byte{},
		cache:          cache.NewLinearCache(apiType),
		logger:         ctrl.Log.WithName("Server"),
	}
	svr.Server = server.NewServer(ctx, svr.cache, svr)
	for _, p := range providers {
		key := p.ProviderKey()
		svr.imageProviders[key] = p
		svr.logger.Info("image provider configured", "key", key)
	}
	return svr, nil
}

func (s *Server) StreamExtensionConfigs(stream extensionservice.ExtensionConfigDiscoveryService_StreamExtensionConfigsServer) error {
	return s.Server.StreamHandler(stream, apiType)
}

func (s *Server) DeltaExtensionConfigs(_ extensionservice.ExtensionConfigDiscoveryService_DeltaExtensionConfigsServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

func (s *Server) FetchExtensionConfigs(ctx context.Context, req *discovery.DiscoveryRequest) (*discovery.DiscoveryResponse, error) {
	req.TypeUrl = apiType
	return s.Server.Fetch(ctx, req)
}
