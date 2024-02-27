package filter2

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/corazawaf/coraza/v3/debuglog"
	ctypes "github.com/corazawaf/coraza/v3/types"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
	"math"
	"net"
	"strconv"
)

const noGRPCStream int32 = -1
const defaultInterruptionStatusCode int = 403

func (ctx *MyHttpContext) handleInterruption(phase interruptionPhase, interruption *ctypes.Interruption) types.Action {
	if ctx.interruptAtPhase.isInterrupted() {
		// handleInterruption should never be called more than once
		panic("Interruption already handled")
	}

	ctx.metrics.CountInterruptionAdd(interruption.RuleID, phase.String(), ctx.metricLabelsKV)

	ctx.logger.Info().
		Str("action", interruption.Action).
		Str("phase", phase.String()).
		Msg("Transaction interrupted")

	ctx.interruptAtPhase = phase
	if phase == interruptPhaseHttpRespBody {
		return replaceResponseBodyWhenInterrupted(ctx.logger, ctx.bodyReadIndex)
	}

	statusCode := interruption.Status
	if statusCode == 0 {
		statusCode = defaultInterruptionStatusCode
	}
	if err := proxywasm.SendHttpResponse(uint32(statusCode), nil, nil, noGRPCStream); err != nil {
		panic(err)
	}

	// SendHttpResponse must be followed by ActionPause in order to stop malicious content
	return types.ActionPause
}

// retrieveAddressInfo retrieves address properties from the proxy
// Expected targets are "source" or "destination"
// Envoy ref: https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/advanced/attributes#connection-attributes
func retrieveAddressInfo(target string, logger debuglog.Logger) (string, int) {
	var targetIP, targetPortStr string
	var targetPort int
	targetAddressRaw, err := proxywasm.GetProperty([]string{target, "address"})
	if err != nil {
		logger.Debug().
			Err(err).
			Msg(fmt.Sprintf("Failed to get %s address", target))
	} else {
		targetIP, targetPortStr, err = net.SplitHostPort(string(targetAddressRaw))
		if err != nil {
			logger.Debug().
				Err(err).
				Msg(fmt.Sprintf("Failed to parse %s address", target))
		}
	}
	targetPortRaw, err := proxywasm.GetProperty([]string{target, "port"})
	if err == nil {
		targetPort, err = parsePort(targetPortRaw)
		if err != nil {
			logger.Debug().
				Err(err).
				Msg(fmt.Sprintf("Failed to parse %s port", target))
		}
	} else if targetPortStr != "" {
		// If GetProperty fails we rely on the port inside the Address property
		// Mostly useful for proxies other than Envoy
		targetPort, err = strconv.Atoi(targetPortStr)
		if err != nil {
			logger.Debug().
				Err(err).
				Msg(fmt.Sprintf("Failed to get %s port", target))

		}
	}
	return targetIP, targetPort
}

// parsePort converts port, retrieved as little-endian bytes, into int
func parsePort(b []byte) (int, error) {
	// Port attribute ({"source", "port"}) is populated as uint64 (8 byte)
	// Ref: https://github.com/envoyproxy/envoy/blob/1b3da361279a54956f01abba830fc5d3a5421828/source/common/network/utility.cc#L201
	if len(b) < 8 {
		return 0, errors.New("port bytes not found")
	}
	// 0 < Port number <= 65535, therefore the retrieved value should never exceed 16 bits
	// and correctly fit int (at least 32 bits in size)
	unsignedInt := binary.LittleEndian.Uint64(b)
	if unsignedInt > math.MaxInt32 {
		return 0, errors.New("port conversion error")
	}
	return int(unsignedInt), nil
}

// replaceResponseBodyWhenInterrupted address an interruption raised during phase 4.
// At this phase, response headers are already sent downstream, therefore an interruption
// can not change anymore the status code, but only tweak the response body
func replaceResponseBodyWhenInterrupted(logger debuglog.Logger, bodySize int) types.Action {
	// TODO(M4tteoP): Update response body interruption logic after https://github.com/corazawaf/coraza-proxy-wasm/issues/26
	// Currently returns a body filled with null bytes that replaces the sensitive data potentially leaked
	err := proxywasm.ReplaceHttpResponseBody(bytes.Repeat([]byte("\x00"), bodySize))
	if err != nil {
		logger.Error().Err(err).Msg("Failed to replace response body")
		return types.ActionContinue
	}
	logger.Warn().Msg("Response body intervention occurred: body replaced")
	return types.ActionContinue
}
