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
	"fmt"
	"net/http"

	"github.com/containerd/containerd/remotes"
	"github.com/deislabs/oras/pkg/auth"
	"github.com/deislabs/oras/pkg/auth/docker"
	"github.com/sirupsen/logrus"
)

// TODO(mathetake): should we allow?
const useInsecure = false

func NewAuthenticator() auth.Client {
	a, err := docker.NewClient()
	if err != nil {
		logrus.Fatalf("error initializing authenticator: %v", err)
	}
	return a
}

func NewResolver(at auth.Client) (remotes.Resolver, error) {
	client := http.DefaultClient
	// TODO(mathetake): set timeout

	// (mathetake): note that the first argument seems not to be used inside of the library
	resolver, err := at.Resolver(context.Background(), client, useInsecure)
	if err != nil {
		return nil, fmt.Errorf("error initializing resolver: %v", err)
	}
	return resolver, nil
}
