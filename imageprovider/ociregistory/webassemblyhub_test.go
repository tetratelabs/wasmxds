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
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mathetake/gasm/wasm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebAssemblyHub_Pull(t *testing.T) {
	t.Run("without login", func(t *testing.T) {
		puller := NewWebAssemblyHub("", "")
		image, err := puller.Fetch(context.Background(),
			"webassemblyhub.io/mathetake/example:v0.1")
		require.NoError(t, err)
		require.Greater(t, len(image), 0)
		module, err := wasm.DecodeModule(bytes.NewReader(image))
		require.NoError(t, err)

		var proxyWasmExported bool
		for _, exp := range module.SecExports {
			t.Log(exp.Name)
			if strings.Contains(exp.Name, "proxy_") {
				proxyWasmExported = true
			}
		}
		assert.True(t, proxyWasmExported)
	})

	t.Run("with login", func(t *testing.T) {
		username := os.Getenv("WEBASSEMBLY_HUB_USERNAME")
		password := os.Getenv("WEBASSEMBLY_HUB_PASSWORD")
		if len(username) == 0 || len(password) == 0 {
			t.Log("to test login against webassemblyhub, you must set" +
				" WEBASSEMBLY_HUB_USERNAME and WEBASSEMBLY_HUB_PASSWORD environment variables")
			return
		}

		puller := NewWebAssemblyHub(username, password)
		image, err := puller.Fetch(context.Background(),
			"webassemblyhub.io/mathetake/example:v0.1")
		require.NoError(t, err)
		require.Greater(t, len(image), 0)
		module, err := wasm.DecodeModule(bytes.NewReader(image))
		require.NoError(t, err)

		var proxyWasmExported bool
		for _, exp := range module.SecExports {
			t.Log(exp.Name)
			if strings.Contains(exp.Name, "proxy_") {
				proxyWasmExported = true
			}
		}
		assert.True(t, proxyWasmExported)
	})
}
