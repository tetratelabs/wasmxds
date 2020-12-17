package main

import (
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
)

func main() {
	proxywasm.SetNewHttpContext(newHttpContext)
	proxywasm.SetNewRootContext(func(contextID uint32) proxywasm.RootContext {
		return &rootContext{}
	})
}

type rootContext struct{ proxywasm.DefaultRootContext }

var headers map[string]string

// override
func (ctx rootContext) OnVMStart(vmConfigurationSize int) bool {
	data, err := proxywasm.GetVMConfiguration(vmConfigurationSize)
	if err != nil {
		proxywasm.LogCriticalf("error reading vm configuration: %v", err)
	}
	headers["vm-configuration"] = string(data)
	return true
}

func (ctx rootContext) OnPluginStart(pluginConfigurationSize int) bool {
	data, err := proxywasm.GetPluginConfiguration(pluginConfigurationSize)
	if err != nil {
		proxywasm.LogCriticalf("error reading plugin configuration: %v", err)
	}
	headers["plugin-configuration"] = string(data)
	return true
}

type httpHeaders struct {
	proxywasm.DefaultHttpContext
	contextID uint32
}

func newHttpContext(rootContextID, contextID uint32) proxywasm.HttpContext {
	return &httpHeaders{contextID: contextID}
}

// override
func (ctx *httpHeaders) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	for k, v := range headers {
		if err := proxywasm.SetHttpResponseHeader(k, v); err != nil {
			proxywasm.LogCriticalf("failed to set response header: %v", err)
		}
	}
	return types.ActionContinue
}
