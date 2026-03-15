package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/core/extension/externalfunc"
	externalv1 "github.com/cuihairu/croupier/pkg/pb/croupier/external/v1"
	"google.golang.org/protobuf/proto"
)

type externalCallFunc func(ctx context.Context, provider, method string, request []byte) ([]byte, error)

func invokeExternalPlatformFunction(
	ctx context.Context,
	functionID string,
	payload []byte,
	call externalCallFunc,
) ([]byte, bool, error) {
	provider, method, ok := parseExternalFunctionID(functionID)
	if !ok {
		return nil, false, nil
	}
	if call == nil {
		return nil, true, errors.New("external platform caller is not configured")
	}

	requestPayload := payload
	protoMode := false
	req := &externalv1.CallPlatformRequest{}
	if err := proto.Unmarshal(payload, req); err == nil {
		if strings.TrimSpace(req.GetPlatform()) != "" || strings.TrimSpace(req.GetMethod()) != "" || len(req.GetRequest()) > 0 {
			protoMode = true
			if p := strings.TrimSpace(req.GetPlatform()); p != "" {
				provider = p
			}
			if m := strings.TrimSpace(req.GetMethod()); m != "" {
				method = m
			}
			requestPayload = req.GetRequest()
		}
	}

	if strings.TrimSpace(provider) == "" || strings.TrimSpace(method) == "" {
		return nil, true, fmt.Errorf("invalid external function id: %s", functionID)
	}

	response, err := call(ctx, provider, method, requestPayload)
	if !protoMode {
		return response, true, err
	}
	resp := &externalv1.CallPlatformResponse{}
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Response = response
	}
	out, marshalErr := proto.Marshal(resp)
	if marshalErr != nil {
		return nil, true, marshalErr
	}
	return out, true, nil
}

func parseExternalFunctionID(functionID string) (provider string, method string, ok bool) {
	return externalfunc.ParseFunctionID(functionID)
}
