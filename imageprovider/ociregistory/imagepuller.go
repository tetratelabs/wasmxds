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

package ociregistory

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/deislabs/oras/pkg/auth"
	"github.com/deislabs/oras/pkg/content"
	"github.com/deislabs/oras/pkg/oras"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"

	wasmxdsv1alpha1 "github.com/tetratelabs/wasmxds/api/v1alpha1"
)

func init() {
	// suppress warning of using artifact spec from oras
	logrus.SetLevel(logrus.ErrorLevel)
}

var (
	ErrAuthenticationFailure = errors.New("authentication failed")
)

type credentialProvider = func() (username string, password string, err error)

func newImagePuller(host string, cp credentialProvider) *imagePuller {
	authClient := NewAuthenticator()
	return &imagePuller{
		host:               host,
		authClient:         authClient,
		localStore:         content.NewMemoryStore(),
		credentialProvider: cp,
	}
}

type (
	imagePuller struct {
		host               string
		authClient         auth.Client
		resolver           remotes.Resolver
		localStore         *content.Memorystore
		credentialProvider credentialProvider
	}
)

// test purpose
func (p *imagePuller) Host() string {
	return p.host
}

func (p *imagePuller) ProviderKey() string {
	return fmt.Sprintf("%s||%s", wasmxdsv1alpha1.ProtocolOCIImageRegistry, p.host)
}

func (p *imagePuller) login() error {
	username, password, err := p.credentialProvider()
	if err != nil {
		return fmt.Errorf("error generating credentials for %s: %w", p.host, err)
	}

	if username != "" && password != "" {
		if err := p.authClient.Login(context.Background(), p.host, username, password, useInsecure); err != nil {
			return fmt.Errorf("error login to host %s with username %s: %w", p.host, password, err)
		}
	}

	r, err := NewResolver(p.authClient)
	if err != nil {
		return fmt.Errorf("error creating resolver for %s: %w", p.host, err)
	}
	p.resolver = r
	return nil
}

func (p *imagePuller) Fetch(ctx context.Context, uri string) ([]byte, error) {
	return p.pull(ctx, uri, false)
}

var (
	AllowedMediaType = []string{
		// https://github.com/engineerd/wasm-to-oci#how-does-this-work
		"application/vnd.module.wasm.content.layer.v1+wasm",

		// https://github.com/solo-io/wasm-image-spec
		"application/vnd.wasm.content.layer.v1+wasm",
	}
	pullOpts = []oras.PullOpt{
		oras.WithAllowedMediaType(AllowedMediaType...),
		oras.WithPullEmptyNameAllowed(),
	}
)

func (p *imagePuller) pull(ctx context.Context, uri string, retried bool) ([]byte, error) {
	if p.resolver == nil {
		if err := p.login(); err != nil {
			return nil, fmt.Errorf("failed to login: %w", err)
		}
	}

	_, layers, err := oras.Pull(ctx, p.resolver, uri, p.localStore, pullOpts...)
	if err == docker.ErrNoToken || err == docker.ErrInvalidAuthorization {
		if retried {
			return nil, fmt.Errorf("%w: %v", ErrAuthenticationFailure, err)
		}
		// if the authentication fails and this is first try, then login and try again
		p.resolver = nil
		return p.pull(ctx, uri, true)
	} else if err != nil {
		return nil, fmt.Errorf("failed to pull: %v", err)
	}

	if len(layers) != 1 {
		return nil, fmt.Errorf("invalid number of image layers")
	}

	_, image, _ := p.localStore.Get(layers[0])
	return image, nil
}

// For e2e testing purpose
func (p *imagePuller) Push(image []byte, ref string) error {
	if err := p.login(); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}
	desc := p.localStore.Add(ref, AllowedMediaType[0], image)
	_, err := oras.Push(context.Background(), p.resolver, ref, p.localStore,
		[]ocispec.Descriptor{desc})
	if err != nil {
		return fmt.Errorf("failed to push: %v", err)
	}
	return nil
}
