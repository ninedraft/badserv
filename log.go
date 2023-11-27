package main

import (
	"context"
	"log/slog"
)

type slogMeta struct {
	slog.Handler
}

type connIDCtxKey struct{}

type requestIDKey struct{}

func (s *slogMeta) Handle(ctx context.Context, record slog.Record) error {
	reqID, okReqID := ctx.Value(requestIDKey{}).(int64)
	if okReqID {
		record.Add("request_id", reqID)
	}

	connID, okConnID := ctx.Value(connIDCtxKey{}).(int64)
	if okConnID {
		record.Add("conn_id", connID)
	}

	return s.Handler.Handle(ctx, record)
}
