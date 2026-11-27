package fairness

import (
	"context"
	"math"
	"testing"
)

func TestSettingsFallBackToDefaults(t *testing.T) {
	for _, c := range []struct {
		in, want Settings
	}{
		{Settings{TenantShare: 0.25, TenantCampaignMPS: 50}, Settings{TenantShare: 0.25, TenantCampaignMPS: 50}},
		{Settings{TenantShare: 1, TenantCampaignMPS: 0}, Settings{TenantShare: 1, TenantCampaignMPS: 0}}, // 0 = no campaign cap
		{Settings{TenantShare: 0, TenantCampaignMPS: -1}, Defaults},
		{Settings{TenantShare: 1.5, TenantCampaignMPS: math.NaN()}, Defaults},
	} {
		if got := c.in.normalized(); got != c.want {
			t.Errorf("%+v → %+v, want %+v", c.in, got, c.want)
		}
	}
	var nilSource *Source
	if nilSource.Get(context.Background()) != Defaults || NewSource(nil).Get(context.Background()) != Defaults {
		t.Fatal("a source without a database must answer the defaults")
	}
	s := Static(Settings{TenantShare: 0.3, TenantCampaignMPS: 10})
	if s.TenantShare(context.Background()) != 0.3 || s.TenantCampaignMPS(context.Background()) != 10 {
		t.Fatalf("static %+v", s.Get(context.Background()))
	}
}
