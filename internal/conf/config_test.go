package conf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadExplicitConfig(t *testing.T) {
	if _, err := Load("../../config/kratos.yml"); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../config/kratos.yml")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_KRATOS_SIGNING_KEY", "a-test-signing-key-longer-than-32-bytes")
	text := strings.Replace(string(source), "local-development-only-change-this-32-byte-key", "${TEST_KRATOS_SIGNING_KEY}", 1)
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil || c.JWTKey != os.Getenv("TEST_KRATOS_SIGNING_KEY") {
		t.Fatalf("environment config: %#v %v", c, err)
	}
	text = strings.Replace(text, "${TEST_KRATOS_SIGNING_KEY}", "short", 1)
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("weak signing key accepted")
	}
}
