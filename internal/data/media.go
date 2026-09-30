package data

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/conf"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os/exec"
	"strings"
)

type Media struct {
	client *minio.Client
	config conf.Media
}

func NewMedia(c conf.Media) (*Media, error) {
	for _, kind := range []string{"video", "cover", "avatar", "background"} {
		if c.Buckets[kind] == "" {
			return nil, fmt.Errorf("media bucket %s is required", kind)
		}
	}
	client, err := minio.New(c.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(c.AccessKey, c.SecretKey, ""), Secure: c.SSL})
	if err != nil {
		return nil, err
	}
	return &Media{client: client, config: c}, nil
}
func (m *Media) Prepare(ctx context.Context) error {
	for _, name := range m.config.Buckets {
		exists, err := m.client.BucketExists(ctx, name)
		if err != nil {
			return err
		}
		if !exists {
			if err := m.client.MakeBucket(ctx, name, minio.MakeBucketOptions{}); err != nil {
				return err
			}
		}
	}
	// 显式初始化注册流程引用的默认资源，已有资源不覆盖。
	for i := 0; i < 10; i++ {
		if err := m.prepareImage(ctx, "avatar", fmt.Sprintf("default%d.png", i), color.RGBA{uint8(40 + i*15), uint8(120 + i*10), 180, 255}, false); err != nil {
			return err
		}
	}
	return m.prepareImage(ctx, "background", "default_background.jpg", color.RGBA{30, 45, 65, 255}, true)
}
func (m *Media) prepareImage(ctx context.Context, kind, name string, shade color.RGBA, isJPEG bool) error {
	bucket := m.config.Buckets[kind]
	if _, err := m.client.StatObject(ctx, bucket, name, minio.StatObjectOptions{}); err == nil {
		return nil
	} else if minio.ToErrorResponse(err).Code != "NoSuchKey" {
		return err
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 128, 128))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(shade), image.Point{}, draw.Src)
	var content bytes.Buffer
	contentType := "image/png"
	var err error
	if isJPEG {
		contentType = "image/jpeg"
		err = jpeg.Encode(&content, canvas, nil)
	} else {
		err = png.Encode(&content, canvas)
	}
	if err != nil {
		return err
	}
	_, err = m.client.PutObject(ctx, bucket, name, bytes.NewReader(content.Bytes()), int64(content.Len()), minio.PutObjectOptions{ContentType: contentType})
	return err
}
func (m *Media) URL(ctx context.Context, kind, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty %s object name", kind)
	}
	url, err := m.client.PresignedGetObject(ctx, m.config.Buckets[kind], name, m.config.Expiry, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}
func (m *Media) Publish(ctx context.Context, data []byte, video, cover string) error {
	if _, err := m.client.PutObject(ctx, m.config.Buckets["video"], video, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "video/mp4"}); err != nil {
		return err
	}
	url, err := m.URL(ctx, "video", video)
	if err != nil {
		return err
	}
	// 直接传递参数，不拼 shell 命令；请求超时能够终止 FFmpeg。
	command := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-i", url, "-frames:v", "1", "-f", "image2pipe", "-vcodec", "png", "pipe:1")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	image, err := command.Output()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.ReplaceAll(stderr.String(), url, "[media URL]"))
	}
	_, err = m.client.PutObject(ctx, m.config.Buckets["cover"], cover, bytes.NewReader(image), int64(len(image)), minio.PutObjectOptions{ContentType: "image/png"})
	return err
}
func (m *Media) Remove(ctx context.Context, video, cover string) error {
	return errors.Join(m.client.RemoveObject(ctx, m.config.Buckets["video"], video, minio.RemoveObjectOptions{}), m.client.RemoveObject(ctx, m.config.Buckets["cover"], cover, minio.RemoveObjectOptions{}))
}
