package licensetool

import (
	"errors"
	"testing"
	"time"
)

func TestBundleRoundTrip(t *testing.T) {
	b := Bundle{Version: 1, CreatedAt: time.Now().UTC().Truncate(time.Second), SigningKeys: map[string]string{"k1": "c2VlZA=="},
		Config: Config{SigningKID: "k1"}, Ledger: []Entry{{Action: "issued", LicenseID: "lic_1"}}}
	data, err := Seal(b, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(data, "correct horse battery")
	if err != nil || got.SigningKeys["k1"] != "c2VlZA==" || got.Ledger[0].LicenseID != "lic_1" {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	if _, err := Open(data, "wrong horse battery"); !errors.Is(err, ErrBadPassphrase) {
		t.Fatalf("wrong passphrase: %v", err)
	}
	data[len(data)-5] ^= 1
	if _, err := Open(data, "correct horse battery"); err == nil {
		t.Fatal("tampered backup opened")
	}
	if _, err := Seal(b, "short"); err == nil {
		t.Fatal("short passphrase accepted")
	}
}
