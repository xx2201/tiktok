package app

import (
	"context"
	"flag"
	"fmt"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/conf"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/data"
	"github.com/go-kratos/kratos/v2/log"
	"os"
	"time"
)

func Run(name string) error {
	path := flag.String("config", "config/kratos.yml", "configuration file")
	migrate := flag.Bool("migrate", false, "apply database schema migration and exit")
	prepare := flag.Bool("prepare-media", false, "create media buckets and exit")
	flag.Parse()
	c, err := conf.Load(*path)
	if err != nil {
		return err
	}
	if *migrate {
		db, err := data.New(c.Database)
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Migrate()
	}
	if *prepare {
		media, err := data.NewMedia(c.Media)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return media.Prepare(ctx)
	}
	logger := log.With(log.NewStdLogger(os.Stdout), "ts", log.DefaultTimestamp, "caller", log.DefaultCaller, "service", name)
	log.SetLogger(logger)
	application, cleanup, err := Build(context.Background(), name, c, logger)
	if err != nil {
		return err
	}
	defer cleanup()
	return application.Run()
}
func Main(name string) {
	if err := Run(name); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
