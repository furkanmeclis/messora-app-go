package config

import (
	"maps"
	"strings"
	"testing"
)

func TestStorageSettings(t *testing.T) {
	base := map[string]string{
		"POSTGRES_PASSWORD":      "pw",
		"APP_ENCRYPTION_KEY":     validKey,
		"BFF_SHARED_SECRET":      strings.Repeat("x", 32),
		"CENTRIFUGO_API_KEY":     "k",
		"CENTRIFUGO_HMAC_SECRET": "h",
	}
	load := func(extra map[string]string) *Config {
		t.Helper()
		m := maps.Clone(base)
		maps.Copy(m, extra)
		cfg, err := loadFrom(envFrom(m), nil)
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}

	// Defaults: the local driver on the media volume, the signing key
	// derived from the encryption key.
	cfg := load(nil)
	if cfg.Storage.Driver != StorageLocal || cfg.Storage.LocalDir != "/var/lib/messora/media" || len(cfg.Storage.SigningKey) != 32 {
		t.Fatalf("defaults: %+v", cfg.Storage)
	}
	if string(cfg.Storage.SigningKey) == string(cfg.EncryptionKey) {
		t.Fatal("the signing key must be derived, not the encryption key itself")
	}
	if err := cfg.Validate(RoleAPI); err != nil {
		t.Fatal(err)
	}
	// MEDIA_DIR is the older name of STORAGE_LOCAL_DIR.
	if c := load(map[string]string{"MEDIA_DIR": "/old"}); c.Storage.LocalDir != "/old" {
		t.Fatalf("MEDIA_DIR: %q", c.Storage.LocalDir)
	}
	if c := load(map[string]string{"MEDIA_DIR": "/old", "STORAGE_LOCAL_DIR": "/new"}); c.Storage.LocalDir != "/new" {
		t.Fatalf("STORAGE_LOCAL_DIR: %q", c.Storage.LocalDir)
	}
	if c := load(map[string]string{"MEDIA_SIGNING_KEY": strings.Repeat("s", 40)}); string(c.Storage.SigningKey) != strings.Repeat("s", 40) {
		t.Fatal("MEDIA_SIGNING_KEY not used")
	}

	s3ok := map[string]string{"STORAGE_DRIVER": "s3", "S3_REGION": "eu-central-1", "S3_BUCKET_PRIVATE": "messora-media",
		"S3_BUCKET_PUBLIC": "messora-public", "PUBLIC_ASSET_BASE_URL": "https://cdn.messora.tech/"}
	c := load(s3ok)
	if err := c.Validate(RoleWorker); err != nil {
		t.Fatalf("valid s3: %v", err)
	}
	if c.Storage.PublicAssetBaseURL != "https://cdn.messora.tech" || c.Storage.S3.SSE != "AES256" || c.Storage.S3.ForcePathStyle {
		t.Fatalf("s3 settings: %+v", c.Storage)
	}
	// AWS_REGION is honoured when S3_REGION is unset.
	if c := load(map[string]string{"AWS_REGION": "us-east-1"}); c.Storage.S3.Region != "us-east-1" {
		t.Fatal("AWS_REGION fallback")
	}

	bad := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"driver", map[string]string{"STORAGE_DRIVER": "ftp"}, "STORAGE_DRIVER"},
		{"short signing key", map[string]string{"STORAGE_DRIVER": "local", "MEDIA_SIGNING_KEY": "short"}, "MEDIA_SIGNING_KEY"},
		{"no region", map[string]string{"S3_REGION": ""}, "S3_REGION"},
		{"no buckets", map[string]string{"S3_BUCKET_PUBLIC": ""}, "S3_BUCKET_PUBLIC"},
		{"same bucket", map[string]string{"S3_BUCKET_PUBLIC": "messora-media"}, "different buckets"},
		{"half credentials", map[string]string{"S3_ACCESS_KEY_ID": "AKIA"}, "S3_SECRET_ACCESS_KEY"},
		{"endpoint", map[string]string{"S3_ENDPOINT": "minio:9000"}, "S3_ENDPOINT"},
		{"sse", map[string]string{"S3_SSE": "aws:kms"}, "S3_SSE"},
		{"cdn scheme", map[string]string{"PUBLIC_ASSET_BASE_URL": "http://cdn.example.com"}, "https"},
		{"cdn url", map[string]string{"PUBLIC_ASSET_BASE_URL": "cdn.example.com"}, "PUBLIC_ASSET_BASE_URL"},
	}
	for _, b := range bad {
		env := maps.Clone(s3ok)
		maps.Copy(env, b.env)
		err := load(env).Validate(RoleAPI)
		if err == nil || !strings.Contains(err.Error(), b.want) {
			t.Errorf("%s: got %v, want an error about %s", b.name, err, b.want)
		}
	}
	// The scheduler and migrations do not touch storage.
	if err := load(map[string]string{"STORAGE_DRIVER": "ftp"}).Validate(RoleScheduler); err != nil {
		t.Errorf("scheduler validated storage: %v", err)
	}
}
