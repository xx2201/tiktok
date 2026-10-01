package app

import (
	"context"
	"crypto/tls"
	"fmt"
	comment "github.com/bytedance-youthcamp-jbzx/tiktok/api/comment/v1"
	favorite "github.com/bytedance-youthcamp-jbzx/tiktok/api/favorite/v1"
	message "github.com/bytedance-youthcamp-jbzx/tiktok/api/message/v1"
	relation "github.com/bytedance-youthcamp-jbzx/tiktok/api/relation/v1"
	user "github.com/bytedance-youthcamp-jbzx/tiktok/api/user/v1"
	video "github.com/bytedance-youthcamp-jbzx/tiktok/api/video/v1"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/conf"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/data"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/server"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/service"
	"github.com/bytedance-youthcamp-jbzx/tiktok/pkg/jwt"
	etcd "github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"net/url"
	"os"
	"time"
)

func Build(ctx context.Context, name string, c *conf.Config, logger log.Logger) (application *kratos.App, cleanup func(), err error) {
	var closers []func()
	cleanup = func() {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()
	var discovery registry.Discovery
	var registrar registry.Registrar
	if c.Registry.Enabled {
		client, e := clientv3.New(clientv3.Config{Endpoints: c.Registry.Endpoints, DialTimeout: 5 * time.Second})
		if e != nil {
			return nil, cleanup, e
		}
		closers = append(closers, func() { _ = client.Close() })
		r := etcd.New(client)
		discovery, registrar = r, r
	}
	var servers []transport.Server
	if name == "api" {
		clients, connections, e := dialClients(ctx, c, discovery)
		for _, connection := range connections {
			conn := connection
			closers = append(closers, func() { _ = conn.Close() })
		}
		if e != nil {
			return nil, cleanup, e
		}
		httpServer := server.NewHTTP(c, clients, logger)
		if c.HTTP.TLSCert != "" {
			certificate, e := tls.LoadX509KeyPair(c.HTTP.TLSCert, c.HTTP.TLSKey)
			if e != nil {
				return nil, cleanup, e
			}
			httpServer.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}
		}
		servers = append(servers, httpServer)
	} else {
		db, e := data.New(c.Database)
		if e != nil {
			return nil, cleanup, e
		}
		closers = append(closers, func() { _ = db.Close() })
		tokens := jwt.NewJWT([]byte(c.JWTKey))
		var media *data.Media
		var profiles *biz.Profiles
		if name != "message" {
			media, e = data.NewMedia(c.Media)
			if e != nil {
				return nil, cleanup, e
			}
			profiles = biz.NewProfiles(db, db, media)
		}
		grpcServer := server.NewGRPC(c.Services[name].Address, c.Media.MaxBytes, logger)
		endpoint, e := url.Parse("grpc://" + c.Services[name].Endpoint)
		if e != nil {
			return nil, cleanup, e
		}
		// 监听地址与对外注册地址分别配置，避免注册 0.0.0.0。
		kgrpc.Endpoint(endpoint)(grpcServer)
		switch name {
		case "user":
			user.RegisterUserServiceServer(grpcServer, service.NewUserService(biz.NewUserUsecase(db, profiles, tokens), tokens))
		case "video":
			uc := biz.NewVideoUsecase(db, profiles, media, c.Media.MaxBytes, logger)
			closers = append(closers, uc.Close)
			// 收藏及其作者、点赞/关注信息都从主库读，避免副本延迟破坏即时可见。
			primary := db.Primary()
			bookmarkViews := biz.NewVideoUsecase(primary, biz.NewProfiles(primary, primary, media), media, c.Media.MaxBytes, logger)
			bookmarks := biz.NewBookmarkUsecase(primary, bookmarkViews)
			video.RegisterVideoServiceServer(grpcServer, service.NewVideoService(uc, bookmarks, tokens))
		case "comment":
			comment.RegisterCommentServiceServer(grpcServer, service.NewCommentService(biz.NewCommentUsecase(db, profiles), tokens))
		case "favorite":
			actions, e := data.NewActions(ctx, name, c.Redis, c.RabbitMQ, db, logger)
			if e != nil {
				return nil, cleanup, e
			}
			closers = append(closers, func() { _ = actions.Close() })
			servers = append(servers, actions)
			videos := biz.NewVideoUsecase(db, profiles, media, c.Media.MaxBytes, logger)
			favorite.RegisterFavoriteServiceServer(grpcServer, service.NewFavoriteService(biz.NewFavoriteUsecase(db, actions, videos), tokens))
		case "relation":
			cipher, e := data.NewCipher(c.MessageKeys)
			if e != nil {
				return nil, cleanup, e
			}
			actions, e := data.NewActions(ctx, name, c.Redis, c.RabbitMQ, db, logger)
			if e != nil {
				return nil, cleanup, e
			}
			closers = append(closers, func() { _ = actions.Close() })
			servers = append(servers, actions)
			relation.RegisterRelationServiceServer(grpcServer, service.NewRelationService(biz.NewRelationUsecase(db, actions, profiles, cipher), tokens))
		case "message":
			cipher, e := data.NewCipher(c.MessageKeys)
			if e != nil {
				return nil, cleanup, e
			}
			message.RegisterMessageServiceServer(grpcServer, service.NewMessageService(biz.NewMessageUsecase(db, cipher), tokens))
		default:
			return nil, cleanup, fmt.Errorf("unknown service %s", name)
		}
		servers = append(servers, grpcServer)
	}
	hostname, _ := os.Hostname()
	options := []kratos.Option{kratos.ID(fmt.Sprintf("%s-%s-%d", hostname, name, os.Getpid())), kratos.Name(c.Services[name].Name), kratos.Logger(logger), kratos.Server(servers...), kratos.StopTimeout(4 * time.Minute)}
	if registrar != nil {
		options = append(options, kratos.Registrar(registrar))
	}
	return kratos.New(options...), cleanup, nil
}
func dialClients(ctx context.Context, c *conf.Config, discovery registry.Discovery) (server.Clients, []*grpc.ClientConn, error) {
	var clients server.Clients
	var connections []*grpc.ClientConn
	for _, name := range []string{"user", "video", "comment", "favorite", "relation", "message"} {
		endpoint := c.Services[name].Endpoint
		options := []kgrpc.ClientOption{kgrpc.WithTimeout(30 * time.Second), kgrpc.WithOptions(grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(c.Media.MaxBytes+(1<<20)), grpc.MaxCallSendMsgSize(c.Media.MaxBytes+(1<<20))))}
		if discovery != nil {
			endpoint = "discovery:///" + c.Services[name].Name
			options = append(options, kgrpc.WithDiscovery(discovery))
		}
		options = append(options, kgrpc.WithEndpoint(endpoint))
		conn, err := kgrpc.DialInsecure(ctx, options...)
		if err != nil {
			return clients, connections, err
		}
		connections = append(connections, conn)
		switch name {
		case "user":
			clients.User = user.NewUserServiceClient(conn)
		case "video":
			clients.Video = video.NewVideoServiceClient(conn)
		case "comment":
			clients.Comment = comment.NewCommentServiceClient(conn)
		case "favorite":
			clients.Favorite = favorite.NewFavoriteServiceClient(conn)
		case "relation":
			clients.Relation = relation.NewRelationServiceClient(conn)
		case "message":
			clients.Message = message.NewMessageServiceClient(conn)
		}
	}
	return clients, connections, nil
}
