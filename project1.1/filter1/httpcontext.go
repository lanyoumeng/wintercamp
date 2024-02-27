package filter1

import (
	"net/url"

	"github.com/tidwall/gjson"

	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
)

const (
	//  请求路径
	HttpPath = ":path"
)

type MyHttpContext struct {
	types.DefaultHttpContext
	requiredKeys   []string
	allReqBodySize int
}

var _ types.HttpContext = (*MyHttpContext)(nil)

func NewMyHttpContext() *MyHttpContext {
	return &MyHttpContext{}
}

func (this *MyHttpContext) OnHttpRequestHeaders(int, bool) types.Action {
	contentType, err := proxywasm.GetHttpRequestHeader("content-type")
	if err != nil || contentType != "application/json" {
		//不是json格式， 返回403
		if err := proxywasm.SendHttpResponse(403, nil, []byte("content-type must be provided"), -1); err != nil {
			panic(err)
		}
		// 表示不可继续
		return types.ActionPause
	}

	// 通过 header 获取request path
	hp, err := proxywasm.GetHttpRequestHeader(HttpPath)
	if err != nil {
		proxywasm.LogErrorf("get http path error: %s", err.Error())
	}
	urlParser, err := url.Parse(hp)
	if err != nil {
		proxywasm.LogError(err.Error())

	}
	// 打印请求路径 以及参数
	proxywasm.LogInfof("uri = %s", urlParser.Path)
	// proxywasm.LogInfof("host = %s", urlParser.Host)
	proxywasm.LogInfof("params = %s", urlParser.RawQuery)

	// //通过参数获取用户 以及判断用户是否为admin
	// if user := urlParser.Query().Get("user"); user != "admin" {
	// 	_ = proxywasm.SendHttpResponse(401,
	// 		[][2]string{
	// 			{"content-type", "application/json; charset=utf-8"},
	// 		},
	// 		[]byte("The user does not have permission or is missing parameters"),
	// 		-1)
	// 	// 表示不可继续
	// 	return types.ActionPause
	// }

	//表示可以继续
	return types.ActionContinue
}

func (this *MyHttpContext) OnHttpRequestBody(ReqBodySize int, endOfStream bool) types.Action {
	this.allReqBodySize += ReqBodySize
	if !endOfStream {
		return types.ActionPause
	}

	body, err := proxywasm.GetHttpRequestBody(0, this.allReqBodySize)
	if err != nil {
		proxywasm.LogErrorf("failed to get request body: %v", err)
		return types.ActionContinue
	}

	// 验证请求体
	if !gjson.ValidBytes(body) {
		proxywasm.LogErrorf("body is not a valid json %v", string(body))
		if err := proxywasm.SendHttpResponse(403, nil, []byte("content-type must be provided"), -1); err != nil {
			panic(err)
		}
		return types.ActionPause
	}
	//解析json
	jsonData := gjson.ParseBytes(body)

	for _, requiredKey := range this.requiredKeys {
		if !jsonData.Get(requiredKey).Exists() {
			proxywasm.LogErrorf("required key (%v) is missing: %v", requiredKey, jsonData)
			if err := proxywasm.SendHttpResponse(403, nil, []byte("content-type must be provided"), -1); err != nil {
				panic(err)
			}
			return types.ActionPause
		}
	}

	return types.ActionContinue

}

func (this *MyHttpContext) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	// 添加响应头
	err := proxywasm.AddHttpResponseHeader("hello", "world")
	if err != nil {
		proxywasm.LogErrorf("add header error: %s", err.Error())
	}
	return types.ActionContinue
}
