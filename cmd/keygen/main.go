package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	directory := flag.String("dir", "config/keys", "local message key directory")
	flag.Parse()
	if err := generate(*directory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func generate(directory string) error {
	publicPath, privatePath := filepath.Join(directory, "message-public.pem"), filepath.Join(directory, "message-private.pem")
	for _, path := range []string{publicPath, privatePath} {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("key already exists: %s; refusing to replace message encryption keys", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	publicBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	privateBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(privatePath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateBytes}), 0600); err != nil {
		return err
	}
	return os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicBytes}), 0644)
}
