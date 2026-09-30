package server

import (
	"context"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	grpc "google.golang.org/grpc"
	"time"
)

func NewGRPC(address string, maxBytes int, logger log.Logger) *kgrpc.Server {
	return kgrpc.NewServer(kgrpc.Address(address), kgrpc.Timeout(30*time.Second), kgrpc.Options(grpc.MaxRecvMsgSize(maxBytes+(1<<20)), grpc.MaxSendMsgSize(maxBytes+(1<<20))), kgrpc.Middleware(recovery.Recovery(), logging.Server(log.NewFilter(logger, log.FilterKey("args"))), safeErrors()))
}
func safeErrors() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			res, err := next(ctx, req)
			if err != nil && errors.Code(err) == 500 {
				log.Context(ctx).Errorf("business operation failed: %v", err)
				return nil, errors.InternalServer("INTERNAL", "服务器内部错误")
			}
			return res, err
		}
	}
}
