package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/runfabric/runfabric/plugin-sdk/go/protocol"
)

type MethodFunc func(ctx context.Context, params json.RawMessage) (any, error)

type Options struct {
	ProtocolVersion string
	Methods         map[string]MethodFunc
}

type Server struct {
	protocolVersion string
	methods         map[string]MethodFunc
}

func New(opts Options) *Server {
	protocolVersion := strings.TrimSpace(opts.ProtocolVersion)
	if protocolVersion == "" {
		protocolVersion = "2025-01-01"
	}
	methods := map[string]MethodFunc{}
	for k, v := range opts.Methods {
		methods[k] = v
	}
	return &Server{
		protocolVersion: protocolVersion,
		methods:         methods,
	}
}

// Serve reads one JSON request per line from in and writes one JSON response per line to out.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	w := bufio.NewWriter(out)
	defer w.Flush()

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req protocol.Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			_ = writeResponseLine(w, protocol.Response{
				ID: "",
				Error: &protocol.ResponseError{
					Code:    "invalid_json",
					Message: err.Error(),
				},
			})
			continue
		}

		res := s.handle(ctx, req)
		if err := writeResponseLine(w, res); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}

func (s *Server) handle(ctx context.Context, req protocol.Request) protocol.Response {
	method := strings.TrimSpace(req.Method)
	if method == "" {
		return protocol.Response{
			ID: req.ID,
			Error: &protocol.ResponseError{
				Code:    "invalid_request",
				Message: "method is required",
			},
		}
	}

	if method == "handshake" {
		return protocol.Response{
			ID: req.ID,
			Result: map[string]any{
				"protocolVersion": s.protocolVersion,
			},
		}
	}

	fn, ok := s.methods[method]
	if !ok {
		return protocol.Response{
			ID: req.ID,
			Error: &protocol.ResponseError{
				Code:    "method_not_found",
				Message: fmt.Sprintf("method %q is not implemented", method),
			},
		}
	}
	result, err := fn(ctx, req.Params)
	if err != nil {
		return protocol.Response{
			ID: req.ID,
			Error: &protocol.ResponseError{
				Code:    "handler_error",
				Message: err.Error(),
			},
		}
	}
	return protocol.Response{
		ID:     req.ID,
		Result: result,
	}
}

func writeResponseLine(w *bufio.Writer, res protocol.Response) error {
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(b, '\n')); err != nil {
		return err
	}
	return w.Flush()
}
