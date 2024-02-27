package filter2

import (
	"github.com/corazawaf/coraza/v3/debuglog"
	"github.com/corazawaf/coraza/v3/types"
	ctypes "github.com/corazawaf/coraza/v3/types"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"io"
)

type logger struct {
	debuglog.Logger
}

var _ debuglog.Logger = &logger{}

var logPrinterFactory = func(io.Writer) debuglog.Printer {
	return func(lvl debuglog.Level, message, fields string) {
		switch lvl {
		case debuglog.LevelUnknown:
			proxywasm.LogDebug("debuglog.LevelUnknown")
		case debuglog.LevelNoLog:
			proxywasm.LogDebug("debuglog.LevelNoLog")
		case debuglog.LevelError:
			proxywasm.LogErrorf("%s %s", message, fields)
		case debuglog.LevelWarn:
			proxywasm.LogWarnf("%s %s", message, fields)
		case debuglog.LevelInfo:
			proxywasm.LogInfof("%s %s", message, fields)
		case debuglog.LevelDebug:
			proxywasm.LogDebugf("%s %s", message, fields)
		case debuglog.LevelTrace:
			proxywasm.LogTracef("%s %s", message, fields)
		default:
		}
	}
}

func DefaultLogger() debuglog.Logger {
	return debuglog.Default()
}

func NewLogger(l debuglog.Logger) {
	debuglog.DefaultWithPrinterFactory(logPrinterFactory)
}
func (l *logger) WithLevel(lvl debuglog.Level) debuglog.Logger {
	return &logger{l.Logger.WithLevel(lvl)}
}

func (l *logger) WithOutput(_ io.Writer) debuglog.Logger {
	proxywasm.LogWarn("Ignoring SecDebugLog directive, debug logs are always routed to proxy logs")
	return l
}

func logError(error types.MatchedRule) {
	msg := error.ErrorLog()
	switch error.Rule().Severity() {
	case ctypes.RuleSeverityEmergency:
		proxywasm.LogCritical(msg)
	case ctypes.RuleSeverityAlert:
		proxywasm.LogCritical(msg)
	case ctypes.RuleSeverityCritical:
		proxywasm.LogCritical(msg)
	case ctypes.RuleSeverityError:
		proxywasm.LogError(msg)
	case ctypes.RuleSeverityWarning:
		proxywasm.LogWarn(msg)
	case ctypes.RuleSeverityNotice:
		proxywasm.LogInfo(msg)
	case ctypes.RuleSeverityInfo:
		proxywasm.LogInfo(msg)
	case ctypes.RuleSeverityDebug:
		proxywasm.LogDebug(msg)
	}
}
