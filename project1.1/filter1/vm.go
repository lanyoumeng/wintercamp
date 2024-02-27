package filter1

import (
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
)

type MyVM struct {
	types.DefaultVMContext
}

func NewMyVM() *MyVM {
	return &MyVM{}
}

func (this *MyVM) OnVMStart(vmConfigurationSize int) types.OnVMStartStatus {
	proxywasm.LogInfo("vm start filter 1")

	return types.OnVMStartStatusOK
}

func (this *MyVM) NewPluginContext(contextID uint32) types.PluginContext {
	return NewHttpPluginContext()
}
