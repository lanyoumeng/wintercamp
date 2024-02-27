package filter2

import (
	"errors"
	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/debuglog"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
	"strings"
)

// MyPluginContext 插件上下文
type MyPluginContext struct {
	types.DefaultPluginContext
	perHostWafs  wafMap
	metricLabels map[string]string
	metrics      *metrics
}

// waf
type wafMap struct {
	kv         map[string]coraza.WAF
	defaultWAF coraza.WAF
}

func newWafMap(capacity int) wafMap {
	return wafMap{
		kv: make(map[string]coraza.WAF, capacity),
	}
}

func (m *wafMap) put(key string, waf coraza.WAF) error {
	if len(key) == 0 {
		return errors.New("empty WAF key")
	}

	m.kv[key] = waf
	return nil
}

func (m *wafMap) setDefaultWAF(w coraza.WAF) {
	if w == nil {
		panic("nil WAF set as default")
	}
	m.defaultWAF = w
}

// string获取coraza.WAF,如果没有找到，则返回默认值
func (m *wafMap) getWAFOrDefault(key string) (coraza.WAF, bool, error) {
	if w, ok := m.kv[key]; ok {
		return w, false, nil
	}

	if m.defaultWAF == nil {
		return nil, false, errors.New("no default WAF")
	}

	return m.defaultWAF, true, nil
}

func NewMyPluginContext() *MyPluginContext {
	return &MyPluginContext{}
}

// OnPluginStart  插件启动
func (this *MyPluginContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	data, err := proxywasm.GetPluginConfiguration()
	if err != nil {
		proxywasm.LogCriticalf("failed to get plugin configuration: %v", err)
		return types.OnPluginStartStatusFailed
	}
	config, err := parsePluginConfig(data, proxywasm.LogInfo)
	if err != nil {
		proxywasm.LogCriticalf("failed to parse plugin configuration: %v", err)
		return types.OnPluginStartStatusFailed
	}

	// key是ruleName，value是host
	hostDirectiveMap := make(map[string][]string)
	for host, directivesRule := range config.hostDirectiveMap {
		hostDirectiveMap[directivesRule] = append(hostDirectiveMap[directivesRule], host)
	}

	//type wafMap struct {
	//	kv         map[string]coraza.WAF
	//	defaultWAF coraza.WAF
	//}
	perHostWafs := newWafMap(len(config.directives))

	// rlueName是指令名称，ruleValue是指令值
	for ruleName, ruleValue := range config.directives {
		var hosts []string
		// 如果ruleName不是默认值，则获取host
		if ruleName != config.defaultDictive {
			var ok bool
			hosts, ok = hostDirectiveMap[ruleName]
			if !ok {
				continue
			}
		}
		conf := coraza.NewWAFConfig().WithErrorCallback(logError).
			WithDebugLogger(debuglog.DefaultWithPrinterFactory(logPrinterFactory)).
			WithRootFS(root)

		waf, err := coraza.NewWAF(conf.WithDirectives(strings.Join(ruleValue, "\n")))
		if err != nil {
			proxywasm.LogCriticalf("failed to create WAF: %v", err)
			return types.OnPluginStartStatusFailed
		}

		// 如果hosts为空，则设置默认WAF
		if len(hosts) == 0 {
			perHostWafs.setDefaultWAF(waf)
		}
		for _, host := range hosts {
			if err := perHostWafs.put(host, waf); err != nil {
				proxywasm.LogCriticalf("failed to put WAF: %v", err)
				return types.OnPluginStartStatusFailed
			}

		}
		// 删除hostDirectiveMap中的ruleName
		delete(hostDirectiveMap, ruleName)
	}

	// 如果hostDirectiveMap不为空，则打印日志
	if len(hostDirectiveMap) > 0 {
		for i := 0; i < len(hostDirectiveMap); i++ {
			proxywasm.LogCriticalf("no directive found for hosts: %v", hostDirectiveMap)
		}
		return types.OnPluginStartStatusFailed
	}

	this.perHostWafs = perHostWafs
	this.metricLabels = config.merticsLabels
	this.metrics = NewMetrics()

	return types.OnPluginStartStatusOK
}

func (this *MyPluginContext) NewHttpContext(contextID uint32) types.HttpContext {
	return &MyHttpContext{
		contextId:      contextID,
		perHostWAFs:    this.perHostWafs,
		metricLabelsKV: this.metricLabels,
		metrics:        this.metrics,
	}
}
