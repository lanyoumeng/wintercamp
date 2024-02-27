package filter2

import (
	"bytes"
	"fmt"
	"github.com/tidwall/gjson"
)

type myPluginConfig struct {
	directives       map[string][]string
	defaultDictive   string
	merticsLabels    map[string]string
	hostDirectiveMap map[string]string
}

func parsePluginConfig(data []byte, loggerInfo func(string)) (*myPluginConfig, error) {
	plugconfig := &myPluginConfig{}
	plugconfig.directives = make(map[string][]string)
	plugconfig.merticsLabels = make(map[string]string)
	plugconfig.hostDirectiveMap = make(map[string]string)

	//校验数据
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return plugconfig, nil
	}

	if !gjson.ValidBytes(data) {
		return plugconfig, fmt.Errorf("invalid json: %q", data)
	}

	jsonData := gjson.ParseBytes(data)

	// 获取默认指令
	plugconfig.defaultDictive = jsonData.Get("default_directives").String()

	// 获取所有的指令
	for key, value := range jsonData.Get("directives_map").Map() {
		// 将字符串值添加到切片中
		for _, v := range value.Array() {
			plugconfig.directives[key] = append(plugconfig.directives[key], v.String())
		}

	}
	if plugconfig.directives[plugconfig.defaultDictive] == nil {
		return plugconfig, fmt.Errorf("default directive %q not found in directives_map", plugconfig.defaultDictive)
	}

	// 如果没有指令，则使用rules字段
	if len(plugconfig.directives) == 0 {
		rules := jsonData.Get("rules")

		if rules.Exists() {
			loggerInfo("Defaulting to deprecated 'rules' field")

			plugconfig.defaultDictive = "default"

			var directive []string
			rules.ForEach(func(_, value gjson.Result) bool {
				directive = append(directive, value.String())
				return true
			})
			plugconfig.directives["default"] = directive
		}
	}
	// 获取所有的指标
	for k, v := range jsonData.Get("metric_labels").Map() {
		plugconfig.merticsLabels[k] = v.String()

	}
	// 获取所有的host_directive_map
	for k, v := range jsonData.Get("host_directive_map").Map() {
		plugconfig.hostDirectiveMap[k] = v.String()

	}

	return plugconfig, nil
}
