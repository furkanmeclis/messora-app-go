package ops

import (
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

func user(perms ...string) *principal.Principal {
	sort.Strings(perms)
	return &principal.Principal{Kind: principal.KindUser, ID: uuid.New(), Permissions: perms}
}

func TestResolveFiltersSectionsByPermission(t *testing.T) {
	cases := []struct {
		name      string
		p         *principal.Principal
		requested []string
		want      []string
	}{
		{"agent", user("conversations.read", "conversations.reply", "contacts.read"), nil, []string{SectionConversations}},
		{"read all", user("conversations.read_all"), nil, []string{SectionConversations, SectionActivity}},
		{"marketing", user("campaigns.read", "automations.read"), nil, []string{SectionCampaigns, SectionAutomations}},
		{"admin", user("campaigns.read", "conversations.read", "conversations.read_all", "automations.read", "settings.read"), nil, AllSections},
		{"subset", user("campaigns.read", "settings.read"), []string{SectionAlerts, SectionConversations}, []string{SectionAlerts}},
		{"nothing", user("contacts.read"), nil, []string{}},
		{"api key", &principal.Principal{Kind: principal.KindAPIKey, Permissions: []string{"campaigns.read"}}, nil, []string{}},
		{"nobody", nil, nil, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Resolve(c.p, c.requested); !slices.Equal(got, c.want) {
				t.Fatalf("Resolve = %v, want %v", got, c.want)
			}
		})
	}
}

// Whoever may read a section may subscribe to the hints (and only they).
func TestOpsChannelMatchesSections(t *testing.T) {
	for _, perm := range []string{"campaigns.read", "conversations.read", "conversations.read_all", "automations.read", "settings.read", "contacts.read", "templates.read"} {
		p := user(perm)
		sections := len(Resolve(p, nil)) > 0
		if got := realtime.Authorize(p, realtime.OpsChannel) == nil; got != sections {
			t.Errorf("%s: may subscribe = %v, has a section = %v", perm, got, sections)
		}
	}
}

func TestSinceIDBoundsUUIDv7(t *testing.T) {
	at := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	lo := SinceID(at)
	if lo.Version() != 7 {
		t.Fatalf("version %d", lo.Version())
	}
	// A UUIDv7 made at the instant sorts at or above the bound; one made a
	// millisecond earlier sorts below it.
	cmp := func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) }
	if cmp(SinceID(at.Add(-time.Millisecond)), lo) >= 0 || cmp(SinceID(at.Add(time.Millisecond)), lo) <= 0 {
		t.Fatal("bound is not ordered by time")
	}
	if ms := int64(lo[0])<<40 | int64(lo[1])<<32 | int64(lo[2])<<24 | int64(lo[3])<<16 | int64(lo[4])<<8 | int64(lo[5]); ms != at.UnixMilli() {
		t.Fatalf("timestamp %d, want %d", ms, at.UnixMilli())
	}
}

func TestStartOfDay(t *testing.T) {
	ist, _ := time.LoadLocation("Europe/Istanbul")
	got := StartOfDay(time.Date(2026, 11, 25, 22, 30, 0, 0, time.UTC), ist) // 01:30 on the 26th in Istanbul
	if want := time.Date(2026, 11, 26, 0, 0, 0, 0, ist); !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
