package filter2

import (
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
)

type MyVM struct {
	types.DefaultVMContext
}

func NewMyVM() types.VMContext {
	return &MyVM{}
}

//func (this *MyVM) OnVMStart(vmConfigurationSize int) types.OnVMStartStatus {
//	proxywasm.LogInfo("vm start filter 2")
//
//	return types.OnVMStartStatusOK
//}

func (*MyVM) NewPluginContext(contextID uint32) types.PluginContext {
	return NewMyPluginContext()
}
