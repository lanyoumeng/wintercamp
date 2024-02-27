package filter1

import (
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/tidwall/gjson"
)

var (
	pluginStartCnt = 0
)

type MyPluginContext struct {
	types.DefaultPluginContext
	requiredKeys []string
}

func NewHttpPluginContext() *MyPluginContext {
	return &MyPluginContext{}
}

func (this *MyPluginContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	data, err := proxywasm.GetPluginConfiguration()
	if err != nil {
		proxywasm.LogCriticalf("failed to get plugin configuration: %v", err)
		return types.OnPluginStartStatusFailed
	}
	if len(data) == 0 {
		proxywasm.LogCritical("plugin configuration is empty")
		return types.OnPluginStartStatusFailed
	}

	config := []string{}

	//验证json
	if !gjson.ValidBytes(data) {
		proxywasm.LogCritical("plugin configuration is not a valid json")
		return types.OnPluginStartStatusFailed
	}
	//解析json
	json := gjson.ParseBytes(data)
	//获取json中的requiredKeys
	requiredKeys := json.Get("requiredKeys")
	if !requiredKeys.Exists() {
		proxywasm.LogCritical("requiredKeys is not found in plugin configuration")
		return types.OnPluginStartStatusFailed
	}

	//将requiredKeys转换为[]string
	for _, v := range requiredKeys.Array() {
		config = append(config, v.String())
	}
	this.requiredKeys = config

	pluginStartCnt++
	proxywasm.LogInfof("pluginStartCnt: %d", pluginStartCnt)
	return types.OnPluginStartStatusOK
}

func (this *MyPluginContext) NewHttpContext(contextID uint32) types.HttpContext {
	return &MyHttpContext{requiredKeys: this.requiredKeys}
}
