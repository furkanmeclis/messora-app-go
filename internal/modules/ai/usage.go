package ai

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
)

// Usage summary groups.
var usageGroups = []string{"model", "provider", "purpose", "campaign", "day"}

// maxUsageRange bounds a summary's date range.
const maxUsageRange = 400 * 24 * time.Hour

// UsageGroup is one row of a usage summary.
type UsageGroup struct {
	Key, Label string
	Row        db.AIUsageSummaryRow
}

// UsageSummary is AI usage between two instants.
type UsageSummary struct {
	From, To        time.Time
	GroupBy         string
	Totals          db.AIUsageSummaryRow
	Escalations     int64
	AIConversations int64
	// HandoffRate = Escalations / AIConversations (nil without AI
	// conversations).
	HandoffRate *float64
	Groups      []UsageGroup
	Budget      Budget
}

// Summary computes the usage metrics of [from, to) grouped by groupBy in
// time zone tz (day groups).
func (s *Service) Summary(ctx context.Context, from, to time.Time, groupBy, tz string) (UsageSummary, error) {
	if groupBy == "" {
		groupBy = "day"
	}
	if !slices.Contains(usageGroups, groupBy) {
		return UsageSummary{}, &FieldError{"group_by", "Group by model, provider, purpose, campaign or day."}
	}
	if !to.After(from) || to.Sub(from) > maxUsageRange {
		return UsageSummary{}, &FieldError{"to", "to must be after from, at most 400 days later."}
	}
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return UsageSummary{}, &FieldError{"tz", "Unknown time zone."}
	}
	out := UsageSummary{From: from, To: to, GroupBy: groupBy}
	tot, err := s.q.AIUsageSummary(ctx, db.AIUsageSummaryParams{GroupBy: "total", TimeZone: tz, FromAt: from, ToAt: to})
	if err != nil {
		return out, err
	}
	if len(tot) == 1 {
		out.Totals = tot[0]
	} else {
		out.Totals.Cost = "0"
	}
	out.AIConversations = out.Totals.AiConversations
	if out.Escalations, err = s.q.CountAIEscalations(ctx, db.CountAIEscalationsParams{FromAt: from, ToAt: to}); err != nil {
		return out, err
	}
	if out.AIConversations > 0 {
		r := float64(out.Escalations) / float64(out.AIConversations)
		out.HandoffRate = &r
	}
	rows, err := s.q.AIUsageSummary(ctx, db.AIUsageSummaryParams{GroupBy: groupBy, TimeZone: tz, FromAt: from, ToAt: to})
	if err != nil {
		return out, err
	}
	labels, err := s.usageLabels(ctx, groupBy, rows)
	if err != nil {
		return out, err
	}
	for _, r := range rows {
		out.Groups = append(out.Groups, UsageGroup{Key: r.Key, Label: labels[r.Key], Row: r})
	}
	out.Budget, err = s.BudgetStatus(ctx)
	return out, err
}

// usageLabels names the group keys: provider and campaign names; other
// keys are their own label. An empty campaign key is AI work outside
// campaigns.
func (s *Service) usageLabels(ctx context.Context, groupBy string, rows []db.AIUsageSummaryRow) (map[string]string, error) {
	labels := map[string]string{}
	for _, r := range rows {
		labels[r.Key] = r.Key
	}
	switch groupBy {
	case "provider":
		ps, err := s.q.ListAIProviders(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			if _, ok := labels[p.ID.String()]; ok {
				labels[p.ID.String()] = p.Name
			}
		}
	case "campaign":
		var ids []uuid.UUID
		for _, r := range rows {
			if id, err := uuid.Parse(r.Key); err == nil {
				ids = append(ids, id)
			}
		}
		names, err := s.q.CampaignNames(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			labels[n.ID.String()] = n.Name
		}
		if _, ok := labels[""]; ok {
			labels[""] = "outside_campaigns"
		}
	}
	return labels, nil
}

func totalsToAPI(r db.AIUsageSummaryRow) api.AIUsageTotals {
	cost := trimZeros(r.Cost)
	if cost == "" {
		cost = "0"
	}
	return api.AIUsageTotals{Requests: r.Requests, Replies: r.Replies, Failed: r.Failed, InputTokens: r.InputTokens,
		OutputTokens: r.OutputTokens, CachedTokens: r.CachedTokens, CacheWriteTokens: r.CacheWriteTokens, Cost: cost,
		AvgLatencyMs: float32(r.AvgLatencyMs)}
}

// GetAIUsageSummary implements api.ServerInterface.
func (h *Handler) GetAIUsageSummary(w http.ResponseWriter, r *http.Request, p api.GetAIUsageSummaryParams) {
	group, tz := "", ""
	if p.GroupBy != nil {
		group = string(*p.GroupBy)
	}
	if p.Tz != nil {
		tz = *p.Tz
	}
	sum, err := h.svc.Summary(r.Context(), p.From, p.To, group, tz)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := api.AIUsageSummary{From: sum.From, To: sum.To, GroupBy: sum.GroupBy, Currency: "USD", Totals: totalsToAPI(sum.Totals),
		AiConversations: sum.AIConversations, Escalations: sum.Escalations, Groups: []api.AIUsageGroup{},
		Budget: api.AIBudget{Spent: sum.Budget.Spent, MonthStart: sum.Budget.MonthStart, Exceeded: sum.Budget.Exceeded}}
	if out.Budget.Spent == "" {
		out.Budget.Spent = "0"
	}
	if sum.Budget.Limit != "" {
		l := sum.Budget.Limit
		out.Budget.Limit = &l
	}
	if sum.HandoffRate != nil {
		v := float32(*sum.HandoffRate)
		out.HandoffRate = &v
	}
	for _, g := range sum.Groups {
		t := totalsToAPI(g.Row)
		out.Groups = append(out.Groups, api.AIUsageGroup{Key: g.Key, Label: g.Label, Requests: t.Requests, Replies: t.Replies,
			Failed: t.Failed, InputTokens: t.InputTokens, OutputTokens: t.OutputTokens, CachedTokens: t.CachedTokens,
			CacheWriteTokens: t.CacheWriteTokens, Cost: t.Cost, AvgLatencyMs: t.AvgLatencyMs})
	}
	httpx.JSON(w, r, http.StatusOK, out)
}
