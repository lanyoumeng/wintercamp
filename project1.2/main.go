package main

import (
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"project1.2/filter2"
	"project1.2/internal/operators"
)

func main() {

	operators.Register()
	proxywasm.SetVMContext(filter2.NewMyVM())
}
