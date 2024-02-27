package main

import (
	"filter/filter1"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
)

func main() {

	proxywasm.SetVMContext(filter1.NewMyVM())
}
