package filter2

import (
	"github.com/corazawaf/coraza/v3/debuglog"
	ctypes "github.com/corazawaf/coraza/v3/types"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
	"net"
	"strconv"
)

const (
	interPhaseNone               = iota
	interruptPhaseHttpReqHeader  = iota
	interruptPhaseHttpReqBody    = iota
	interruptPhaseHttpRespHeader = iota
	interruptPhaseHttpRespBody   = iota
)

type interruptionPhase int8

func (p interruptionPhase) isInterrupted() bool {
	return p != interPhaseNone
}

func (p interruptionPhase) String() string {
	switch p {
	case interruptPhaseHttpReqHeader:
		return "http_request_headers"
	case interruptPhaseHttpReqBody:
		return "http_request_body"
	case interruptPhaseHttpRespHeader:
		return "http_response_headers"
	case interruptPhaseHttpRespBody:
		return "http_response_body"
	default:
		return "no interruption yet"
	}
}

type MyHttpContext struct {
	types.DefaultHttpContext
	contextId      uint32
	perHostWAFs    wafMap
	metrics        *metrics
	metricLabelsKV map[string]string //+

	httpProtocol      string //+
	processedReqBody  bool   // 请求体是否已处理 +
	processedRespBody bool   // 响应体是否已处理  +
	bodyReadIndex     int    //+

	transaction      ctypes.Transaction // 事务
	interruptAtPhase interruptionPhase
	logger           debuglog.Logger
}

var _ types.HttpContext = (*MyHttpContext)(nil)

func (this *MyHttpContext) OnHttpRequestHeaders(int, bool) types.Action {
	defer logTime("OnHttpRequestHeaders", onTime())
	this.metrics.CountAdd()

	//HTTP/2 要求请求具有 :authority 伪标头或 host 标头。
	//当直接构建 HTTP/2 请求时首选 :authority，从 HTTP/1 转换时首选 host（例如在代理中）。
	//如果 :authority 不存在，则兼容性 API 将回退到 host。
	//有关详细信息，请参阅 request.authority。 但是，如果不使用兼容性 API
	//（或直接使用 req.headers），则需要自己实现任何回退行为。
	//:authority： www.cnblogs.com
	//Host： 指定请求的服务器的域名和端口号
	authority, err := proxywasm.GetHttpRequestHeader(":authority")

	if err != nil {
		proxywasm.LogErrorf("get http host error: %s", err.Error())
		requestHost, err := proxywasm.GetProperty([]string{"request", "host"})
		if err != nil {
			proxywasm.LogWarnf("Failed to get the :authority pseudo-header or property of host of the request: %v", err)
			return types.ActionContinue
		}
		authority = string(requestHost)
	}

	//获取waf
	waf, isDefault, err := this.perHostWAFs.getWAFOrDefault(authority)
	if err != nil {
		proxywasm.LogErrorf("get waf error: %s", err.Error())
		return types.ActionContinue
	}
	//创建事务
	this.transaction = waf.NewTransaction()

	//调试日志记录器设置为与事务的调试日志记录器相关联
	logFields := []debuglog.ContextField{debuglog.Uint("context_id", uint(this.contextId))}
	if isDefault {
		logFields = append(logFields, debuglog.Str("authority", authority))
	}
	this.logger = this.transaction.DebugLogger().With(logFields...)
	//获取host
	host, _, err := net.SplitHostPort(authority)
	if err != nil {
		// missing port or bad format
		this.logger.Debug().
			Err(err).
			Msg("Failed to parse server name from authority")
		host = authority
	}

	this.transaction.AddRequestHeader("host", host)
	this.transaction.SetServerName(host)

	if !isDefault {
		this.metricLabelsKV["host"] = host
	}

	transaction := this.transaction

	if transaction.IsRuleEngineOff() {
		// 如果规则引擎关闭，不执行规则
		this.logger.Debug().Msg("Rule engine is off when processing request headers")
		return types.ActionContinue

	}
	//1处理连接
	sourIp, sourPort := retrieveAddressInfo("source", this.logger)
	destIp, destPort := retrieveAddressInfo("destination", this.logger)
	transaction.ProcessConnection(sourIp, sourPort, destIp, destPort)

	//2处理URI
	//GET /path/to/resource HTTP/1.1
	//Host: www.example.com
	//User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:97.0) Gecko/20100101 Firefox/97.0
	//Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8
	//Accept-Language: en-US,en;q=0.5
	uri, err := proxywasm.GetHttpRequestHeader(":path")
	if err != nil {
		this.logger.Error().Err(err).Msg("Failed to get the :path pseudo-header of the request")
		propPathRaw, propPathErr := proxywasm.GetProperty([]string{"request", "path"})
		if propPathErr != nil {
			this.logger.Error().
				Err(propPathErr).
				Msg("Failed to get property of path of the request")
			return types.ActionContinue
		}
		uri = string(propPathRaw)

		return types.ActionContinue

	}
	method, err := proxywasm.GetHttpRequestHeader(":method")
	if err != nil {
		this.logger.Error().Err(err).Msg("Failed to get the :method pseudo-header of the request")
		propMethodRaw, propMethodErr := proxywasm.GetProperty([]string{"request", "method"})
		if propMethodErr != nil {
			this.logger.Error().
				Err(propMethodErr).
				Msg("Failed to get property of method of the request")
			return types.ActionContinue
		}
		method = string(propMethodRaw)

		return types.ActionContinue

	}
	//httpVersion, err := proxywasm.GetHttpRequestHeader(":version")
	//if err != nil {
	//	this.logger.Error().Err(err).Msg("Failed to get the :version pseudo-header of the request")
	//	return types.ActionContinue
	//}
	httpVersion, err := proxywasm.GetProperty([]string{"request", "protocol"})
	if err != nil {
		httpVersion = []byte("HTTP/2.0")
	}

	transaction.ProcessURI(uri, method, string(httpVersion))
	//3处理请求头
	reqHeaders, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		proxywasm.LogErrorf("get http request headers error: %s", err.Error())
		return types.ActionContinue
	}
	for _, v := range reqHeaders {
		transaction.AddRequestHeader(v[0], v[1])

	}
	interrupt := transaction.ProcessRequestHeaders()
	if interrupt != nil {
		return this.handleInterruption(interruptPhaseHttpReqHeader, interrupt)
	}

	return types.ActionContinue
}

func (this *MyHttpContext) OnHttpRequestBody(ReqBodySize int, endOfStream bool) types.Action {
	defer logTime("OnHttpRequestBody", onTime())
	//如果在此之前已经中断，则跳过请求体处理
	if this.interruptAtPhase.isInterrupted() {
		this.logger.Error().Str("in", this.interruptAtPhase.String()).Msg("Request body processing skipped due to previous interruption")
		return types.ActionPause
	}
	if this.processedReqBody {
		this.logger.Debug().Msg("Request body processing skipped due to previous processing")
		return types.ActionContinue

	}
	if this.transaction == nil {
		this.logger.Error().Msg("Transaction is nil when processing request body")
		return types.ActionContinue
	}
	transaction := this.transaction
	if transaction.IsRuleEngineOff() {
		// 如果规则引擎关闭，不执行规则
		this.logger.Debug().Msg("Rule engine is off when processing request body")
		return types.ActionContinue
	}

	if !transaction.IsRequestBodyAccessible() {
		this.logger.Debug().Msg("Request body access is disabled")
		// 即使在 SecRequestBodyAccess 被设置为 false 的情况下，仍会执行 ProcessRequestBody 方法
		this.processedReqBody = true
		interrupt, err := transaction.ProcessRequestBody()
		if err != nil {
			this.logger.Error().Err(err).Msg("Failed to process the request body")
			return types.ActionContinue
		}
		if interrupt != nil {
			return this.handleInterruption(interruptPhaseHttpReqBody, interrupt)
		}
		return types.ActionContinue
	}

	//计算当前数据块的大小
	nowSize := ReqBodySize - this.bodyReadIndex

	//如果当前数据块的大小大于0
	if nowSize > 0 {
		bodySizeNowRead, err := proxywasm.GetHttpRequestBody(this.bodyReadIndex, nowSize)
		if err != nil {
			this.logger.Error().Err(err).Msg("Failed to read the request body")
			return types.ActionContinue
		}
		//读取的数据块的大小和当前数据块的大小不一致
		if len(bodySizeNowRead) != nowSize {
			this.logger.Error().Msg("Failed to read the request body")

		}
		//写入请求体
		interrupt, writeSize, err := transaction.WriteRequestBody(bodySizeNowRead)
		if err != nil {
			this.logger.Error().Err(err).Msg("Failed to write the request body")
			return types.ActionContinue
		}
		//如果中断不为空
		if interrupt != nil {
			return this.handleInterruption(interruptPhaseHttpReqBody, interrupt)
		}

		// 实际写入的字节数小于读取到的数据块的大小，说明已经达到了请求体限制
		if writeSize < len(bodySizeNowRead) {
			//表示已经处理完整个请求体。
			this.processedReqBody = true
			return types.ActionContinue
		}

		//更新已读取的数据块的大小
		this.bodyReadIndex += writeSize
	}

	if endOfStream {
		//表示已经处理完整个请求体。
		this.processedReqBody = true
		// 重置已读取的数据块的大小
		this.bodyReadIndex = 0

		interrupt, err := transaction.ProcessRequestBody()
		if err != nil {
			this.logger.Error().Err(err).Msg("Failed to process the request body")
			return types.ActionContinue
		}
		if interrupt != nil {
			return this.handleInterruption(interruptPhaseHttpReqBody, interrupt)

		}

	}

	return types.ActionContinue
}

func (this *MyHttpContext) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	defer logTime("OnHttpResponseHeaders", onTime())
	if this.interruptAtPhase.isInterrupted() {
		this.logger.Error().Str("in", this.interruptAtPhase.String()).Msg("Response headers processing skipped due to previous interruption")
		return types.ActionContinue

	}
	if this.transaction == nil {
		this.logger.Error().Msg("Transaction is nil when processing response headers")
		return types.ActionContinue
	}
	transaction := this.transaction
	if transaction.IsRuleEngineOff() {
		// 如果规则引擎关闭，不执行规则
		this.logger.Debug().Msg("Rule engine is off when processing response headers")
		return types.ActionContinue

	}
	//确保即使请求没有请求主体，仍然执行了请求主体处理阶段的规则
	if !this.processedReqBody {
		this.processedReqBody = true

		interupt, err := transaction.ProcessRequestBody()

		if err != nil {
			this.logger.Error().Err(err).Msg("Failed to process the request body")
			return types.ActionContinue

		}
		if interupt != nil {
			return this.handleInterruption(interruptPhaseHttpReqBody, interupt)
		}
	}
	status, err := proxywasm.GetHttpResponseHeader(":status")
	if err != nil {
		this.logger.Error().Err(err).Msg("Failed to get the :status pseudo-header of the response")
		propCodeRaw, err := proxywasm.GetProperty([]string{"response", "code"})
		if err != nil {
			this.logger.Error().
				Err(err).
				Msg("Failed to get property of code of the response")
			return types.ActionContinue
		}
		status = string(propCodeRaw)

	}
	code, err := strconv.Atoi(status)
	if err != nil {
		code = 0
	}

	hs, err := proxywasm.GetHttpResponseHeaders()
	if err != nil {
		this.logger.Error().
			Err(err).
			Msg("Failed to get response headers")
		return types.ActionContinue
	}

	for _, h := range hs {
		transaction.AddResponseHeader(h[0], h[1])
	}

	interrupt := transaction.ProcessResponseHeaders(code, this.httpProtocol)
	if interrupt != nil {
		return this.handleInterruption(interruptPhaseHttpRespHeader, interrupt)

	}
	return types.ActionContinue
}

func (this *MyHttpContext) OnHttpResponseBody(ResBodySize int, endOfStream bool) types.Action {
	defer logTime("OnHttpResponseBody", onTime())

	//如果在此之前已经中断，则跳过响应体处理
	if this.interruptAtPhase.isInterrupted() {
		this.logger.Error().Str("in", this.interruptAtPhase.String()).Msg("Response body processing skipped due to previous interruption")
		return replaceResponseBodyWhenInterrupted(this.logger, ResBodySize)

	}
	if this.processedRespBody {
		this.logger.Debug().Msg("interrupt happen ,Response body do not clear all")
		return types.ActionContinue

	}
	if this.transaction == nil {
		this.logger.Error().Msg("Transaction is nil when processing response body")
		return types.ActionContinue

	}
	transaction := this.transaction
	if transaction.IsRuleEngineOff() {
		// 如果规则引擎关闭，不执行规则
		this.logger.Debug().Msg("Rule engine is off when processing response body")
		return types.ActionContinue

	}

	//
	if !transaction.IsResponseBodyAccessible() {
		this.logger.Debug().Msg("Response body access is disabled")

		if !this.processedRespBody {
			this.processedRespBody = true
			interrupt, err := transaction.ProcessResponseBody()
			if err != nil {
				this.logger.Error().Err(err).Msg("Failed to process the response body")
				return types.ActionContinue
			}
			if interrupt != nil {
				return this.handleInterruption(interruptPhaseHttpRespBody, interrupt)
			}

		}
		return types.ActionContinue

	}

	//计算当前数据块的大小
	nowSize := ResBodySize - this.bodyReadIndex

	//如果当前数据块的大小大于0
	if nowSize > 0 {
		bodySizeNowRead, err := proxywasm.GetHttpResponseBody(this.bodyReadIndex, nowSize)
		if err != nil {
			this.logger.Error().Err(err).Msg("Failed to read the response body")
			return types.ActionContinue
		}
		//读取的数据块的大小和当前数据块的大小不一致
		if len(bodySizeNowRead) != nowSize {
			this.logger.Error().Msg("Failed to read the response body")
		}
		//写入请求体
		interrupt, writeSize, err := transaction.WriteResponseBody(bodySizeNowRead)
		if err != nil {
			this.logger.Error().Err(err).Msg("Failed to write the response body")
			return types.ActionContinue
		}
		//如果中断不为空
		if interrupt != nil {
			return this.handleInterruption(interruptPhaseHttpRespBody, interrupt)
		}

		// 实际写入的字节数小于读取到的数据块的大小，说明已经达到了请求体限制
		if writeSize < len(bodySizeNowRead) {
			//表示已经处理完整个响应体。
			this.processedRespBody = true
			return types.ActionContinue
		}

		//更新已读取的数据块的大小
		this.bodyReadIndex += writeSize
	}

	//表示已经处理完整个响应体。
	if endOfStream {
		this.processedRespBody = true

		interrupt, err := transaction.ProcessResponseBody()
		if err != nil {
			this.logger.Error().Err(err).Msg("Failed to process the response body")
			return types.ActionContinue
		}
		if interrupt != nil {
			return this.handleInterruption(interruptPhaseHttpRespBody, interrupt)
		}

		return types.ActionContinue

	}

	return types.ActionContinue
}

func (this *MyHttpContext) OnHttpStreamDone() {
	defer logTime("OnHttpStreamDone", onTime())
	transaction := this.transaction
	if transaction == nil {
		this.logger.Error().Msg("Transaction is nil when processing stream done")
		return
	}
	if transaction.IsRuleEngineOff() && !this.interruptAtPhase.isInterrupted() {
		if !this.processedReqBody {

			this.processedReqBody = true
			this.logger.Debug().Msg("Request body processing skipped due to stream done")
			_, err := transaction.ProcessRequestBody()
			if err != nil {
				this.logger.Error().Err(err).Msg("Failed to process the request body")
			}
		}

		this.transaction.ProcessLogging()

		this.transaction.Close()

		this.logger.Debug().Msg("Rule engine is off when processing stream done")
		logMemStats()

		return

	}

}
