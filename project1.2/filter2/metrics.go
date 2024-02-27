package filter2

import (
	"fmt"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"strings"
)

type metrics struct {
	// 用于记录请求总数
	countersKv map[string]proxywasm.MetricCounter
	// 用于记录请求耗时
	histogramsKv map[string]proxywasm.MetricHistogram
}

func NewMetrics() *metrics {
	return &metrics{
		countersKv:   make(map[string]proxywasm.MetricCounter),
		histogramsKv: make(map[string]proxywasm.MetricHistogram),
	}
}

// 计数器增加
func (m *metrics) IncreaseCounter(key string) {
	counter, ok := m.countersKv[key]
	if !ok {
		counter = proxywasm.DefineCounterMetric(key)
		m.countersKv[key] = counter
	}
	counter.Increment(1)
}

// 记录直方图
//func (m *metrics) RecordHistogram(key string, value int) {
//	m.histogramsKv[key].RecordValue(value)
//}

// 记录请求总数
func (m *metrics) CountAdd() {
	m.IncreaseCounter("waf_filter.tx.total")
}

// 中断请求总数
func (m *metrics) CountInterruptionAdd(ruleId int, phase string, metrics map[string]string) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("waf_http_request_interruption_total{rule_id=\"%d\",phase=\"%s\"", ruleId, phase))
	for k, v := range metrics {
		sb.WriteString(fmt.Sprintf(",%s=\"%s\"", k, v))

	}
	m.IncreaseCounter(sb.String())
}
