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

package wasmxds

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	wasmxdsv1alpha1 "github.com/tetratelabs/wasmxds/api/v1alpha1"
	v1converter "github.com/tetratelabs/wasmxds/converter/v1alpha1"
)

type EventHandler interface {
	Update(extension *wasmxdsv1alpha1.WasmExtension, pluginConfig, vmConfig string) (ctrl.Result, error)
	Delete(extension *wasmxdsv1alpha1.WasmExtension)
}

var _ EventHandler = &Server{}

func (s *Server) handlerLogger() logr.Logger {
	return s.logger.WithName("EventHandler")
}

func (s *Server) Update(extension *wasmxdsv1alpha1.WasmExtension, pluginConfig, vmConfig string) (res ctrl.Result, err error) {
	s.handlerLogger().Info("updating extension", "name", extension.Namespaced())
	image, ok := s.imageCache[extension.Spec.Image.URI]
	if !ok {
		s.handlerLogger().Info("fetching image", "name", extension.Namespaced(),
			"uri", extension.Spec.Image.URI, "protocol", extension.Spec.Image.Protocol)
		image, err = s.fetchImage(&extension.Spec.Image)
		if err != nil {
			err = fmt.Errorf("failed to fetch image %s: %w", extension.Spec.Image.ID(), err)
			return
		}
	}

	s.handlerLogger().Info("image successfully fetched", "name", extension.Namespaced(),
		"uri", extension.Spec.Image.URI, "protocol", extension.Spec.Image.Protocol)

	if extension.Spec.Image.Sha256 != nil {
		raw := sha256.Sum256(image)
		exp := *extension.Spec.Image.Sha256
		if actual := hex.EncodeToString(raw[:]); actual != exp {
			err = fmt.Errorf("the sha256 value of the fetched image "+
				"differs from the one specified in spec.image.sha256: `%s` != `%s`", actual, exp)
			return
		}
		s.handlerLogger().Info("sha256 check passed", "name", extension.Namespaced())
	} else {
		s.handlerLogger().Info("spec.image.sha256 not specified", "name", extension.Namespaced())
	}

	s.handlerLogger().Info("converting extension to TypedConfiguration", "name", extension.Namespaced())
	tc, err := v1converter.Convert(extension, image, pluginConfig, vmConfig)
	if err != nil {
		return res, fmt.Errorf("invalid extension: %w", err)
	}

	s.handlerLogger().Info("reconciliation successfully finished", "name", extension.Namespaced())
	return res, s.cache.UpdateResource(extension.Namespaced(), tc)
}

func (s *Server) Delete(extension *wasmxdsv1alpha1.WasmExtension) {
	s.handlerLogger().Info("deleting extension", "name", extension.Namespaced())
	delete(s.imageCache, extension.Spec.Image.URI)
	_ = s.cache.DeleteResource(extension.Namespaced())
}

func (s *Server) fetchImage(spec *wasmxdsv1alpha1.WasmExtensionSpecImage) ([]byte, error) {
	key, err := spec.ProviderKey()
	if err != nil {
		return nil, err
	}

	provider, ok := s.imageProviders[key]
	if !ok {
		return nil, fmt.Errorf("the provider image spec not supported: [protocol: %s, uri: %s]",
			spec.Protocol, spec.URI)
	}

	image, err := provider.Fetch(context.Background(), spec.URI)
	if err != nil {
		return nil, fmt.Errorf("error fetching image: %w", err)
	}

	s.imageCache[spec.URI] = image
	return image, nil
}
