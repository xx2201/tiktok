package data

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/conf"
	"os"
)

type Cipher struct {
	public  *rsa.PublicKey
	private *rsa.PrivateKey
}

func NewCipher(c conf.MessageKeys) (*Cipher, error) {
	publicBytes, err := os.ReadFile(c.Public)
	if err != nil {
		return nil, fmt.Errorf("read message public key: %w", err)
	}
	privateBytes, err := os.ReadFile(c.Private)
	if err != nil {
		return nil, fmt.Errorf("read message private key: %w", err)
	}
	publicBlock, _ := pem.Decode(publicBytes)
	privateBlock, _ := pem.Decode(privateBytes)
	if publicBlock == nil || privateBlock == nil {
		return nil, fmt.Errorf("invalid message PEM keys")
	}
	publicAny, err := x509.ParsePKIXPublicKey(publicBlock.Bytes)
	if err != nil {
		return nil, err
	}
	privateAny, err := x509.ParsePKCS8PrivateKey(privateBlock.Bytes)
	if err != nil {
		return nil, err
	}
	public, ok := publicAny.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("message public key must be RSA")
	}
	private, ok := privateAny.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("message private key must be RSA")
	}
	if public.N.Cmp(private.N) != 0 || public.E != private.E || public.Size() < 256 {
		return nil, fmt.Errorf("message RSA keys must match and be at least 2048 bits")
	}
	return &Cipher{public: public, private: private}, nil
}
func (c *Cipher) Encrypt(content string) (string, error) {
	if len(content) > c.public.Size()-11 {
		return "", biz.ErrInvalid
	}
	data, err := rsa.EncryptPKCS1v15(rand.Reader, c.public, []byte(content))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
func (c *Cipher) Decrypt(content string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return "", err
	}
	plaintext, err := rsa.DecryptPKCS1v15(rand.Reader, c.private, data)
	return string(plaintext), err
}
