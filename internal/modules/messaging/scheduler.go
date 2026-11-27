package messaging

import (
	"context"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// The send scheduler (docs/ARCHITECTURE.md §6.3). Every send from a number
// takes a token from that number's GCRA bucket at its current (AIMD) rate R.
// Campaign sends also take a token from the number's campaign lane, at
// R × (1 − reserved share), and from their own campaign's bucket, at the
// campaign's fair share of the lane. All buckets are checked and taken in
// one Lua call, so a refused send consumes nothing. Inbox, API and
// automation sends only need the number bucket: whatever campaigns do, at
// least the reserved share of R stays free for them.
//
// Shares are weighted fair: running campaigns on a number split the lane by
// priority weight (high 4, normal 2, low 1), a campaign's own rate cap is
// honoured and what it leaves is redistributed (water-filling). The
// campaign dispatcher computes the shares on every tick and stores its own
// in Redis for a few seconds; a send whose share is missing (Redis restart,
// a campaign that just started) uses the whole lane, still bounded by it.

// PriorityWeights are the scheduler weights of the campaign priorities.
var PriorityWeights = map[string]float64{"high": 4, "normal": 2, "low": 1}

// WeightOf returns the weight of a priority (normal when unknown).
func WeightOf(priority string) float64 {
	if w, ok := PriorityWeights[priority]; ok {
		return w
	}
	return PriorityWeights["normal"]
}

// DefaultReservedShare is the part of a number's throughput kept for
// inbox, API and automation messages while campaigns send.
const DefaultReservedShare = 0.2

// shareTTL is how long a stored campaign share stays valid; the dispatcher
// refreshes it every tick.
const shareTTL = 15 * time.Second

// Contender is a running campaign competing for a number's campaign lane.
type Contender struct {
	ID     uuid.UUID
	Weight float64
	// Cap is the campaign's own rate limit in messages per second (0 = none).
	Cap float64
}

// FairShares splits capacity (messages per second) among the contenders in
// proportion to their weights, never giving a contender more than its cap;
// capacity a capped contender leaves is shared by the others.
func FairShares(capacity float64, cs []Contender) map[uuid.UUID]float64 {
	out := make(map[uuid.UUID]float64, len(cs))
	active := make([]Contender, 0, len(cs))
	for _, c := range cs {
		if c.Weight <= 0 {
			c.Weight = 1
		}
		active = append(active, c)
	}
	// Smallest cap-to-weight ratio first: those are capped before the rest.
	sort.SliceStable(active, func(i, j int) bool { return ratio(active[i]) < ratio(active[j]) })
	remaining := math.Max(capacity, 0)
	for len(active) > 0 {
		var total float64
		for _, c := range active {
			total += c.Weight
		}
		first := active[0]
		if first.Cap > 0 && first.Cap < remaining*first.Weight/total {
			out[first.ID] = first.Cap
			remaining -= first.Cap
			active = active[1:]
			continue
		}
		for _, c := range active {
			out[c.ID] = remaining * c.Weight / total
		}
		break
	}
	return out
}

func ratio(c Contender) float64 {
	if c.Cap <= 0 {
		return math.Inf(1)
	}
	return c.Cap / c.Weight
}

// Capacity describes a number's throughput as the scheduler sees it.
type Capacity struct {
	// Ceiling is the throughput Meta reports for the number (80 when
	// unknown); Rate is the current AIMD rate (≤ Ceiling).
	Ceiling, Rate int
	// ReservedShare of Rate is kept for inbox, API and automation sends;
	// Campaigns is what campaigns share (messages per second).
	ReservedShare float64
	Campaigns     float64
}

// SetReservedShare sets the part of each number's rate kept for
// non-campaign sends (clamped to 5–90 %).
func (t *Throttle) SetReservedShare(f float64) *Throttle {
	t.reserved = math.Min(math.Max(f, 0.05), 0.9)
	return t
}

// Capacity returns pn's current capacity.
func (t *Throttle) Capacity(ctx context.Context, pn uuid.UUID) Capacity {
	r := t.Rate(ctx, pn)
	return Capacity{Ceiling: t.CeilingFor(ctx, pn), Rate: r, ReservedShare: t.reserved, Campaigns: float64(r) * (1 - t.reserved)}
}

// SetShare stores a campaign's share of pn's campaign lane.
func (t *Throttle) SetShare(ctx context.Context, pn, campaign uuid.UUID, mps float64) {
	_ = t.rdb.Set(ctx, t.shareKey(ctx, pn, campaign), strconv.FormatFloat(mps, 'f', 3, 64), shareTTL).Err()
}

// Share returns a campaign's stored share (0 when none).
func (t *Throttle) Share(ctx context.Context, pn, campaign uuid.UUID) float64 {
	v, err := t.rdb.Get(ctx, t.shareKey(ctx, pn, campaign)).Float64()
	if err != nil {
		return 0
	}
	return v
}

// Keys live under the tenant's Redis prefix, whose hash tag (the tenant
// ID) puts every bucket of one send in one slot, so the Lua call also works
// on a cluster.
func (t *Throttle) numberKey(ctx context.Context, pn uuid.UUID) string {
	return tenant.RedisKey(ctx, "send:pn:"+pn.String())
}
func (t *Throttle) laneKey(ctx context.Context, pn uuid.UUID) string {
	return tenant.RedisKey(ctx, "send:pn:"+pn.String()+":campaigns")
}
func (t *Throttle) campaignKey(ctx context.Context, pn, c uuid.UUID) string {
	return tenant.RedisKey(ctx, "send:pn:"+pn.String()+":c:"+c.String())
}
func (t *Throttle) shareKey(ctx context.Context, pn, c uuid.UUID) string {
	return tenant.RedisKey(ctx, "share:pn:"+pn.String()+":"+c.String())
}

// tenantKey is the bucket of all campaign sends of the tenant.
func (t *Throttle) tenantKey(ctx context.Context) string {
	return tenant.RedisKey(ctx, "send:campaigns")
}

// WithTenantCampaignCap caps the combined campaign send rate of each tenant
// at fn(ctx) messages per second (0 = no cap), across all its numbers: the
// fairness setting TenantCampaignMPS.
func (t *Throttle) WithTenantCampaignCap(fn func(context.Context) float64) *Throttle {
	t.tenantCap = fn
	return t
}

// sendGCRA checks GCRA buckets and takes a token from each only when all of
// them allow it. KEYS[1..n] are buckets, KEYS[n+1] an optional share key
// whose value (messages per second) replaces the rate of the last bucket.
// ARGV: n, then per bucket the rate (per second) and the burst; ARGV after
// the buckets: 1 when KEYS[n+1] is a share key.
var sendGCRA = goredis.NewScript(`
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + tonumber(t[2]) / 1000
local n = tonumber(ARGV[1])
local hasShare = tonumber(ARGV[2 + 2 * n]) == 1
local wait = 0
local tats = {}
for i = 1, n do
  local rate = tonumber(ARGV[2 * i])
  local burst = tonumber(ARGV[2 * i + 1])
  if i == n and hasShare then
    local s = tonumber(redis.call('GET', KEYS[n + 1]) or '')
    if s and s > 0 then
      rate = s
      burst = math.max(1, s)
    end
  end
  local interval = 1000 / rate
  local tat = tonumber(redis.call('GET', KEYS[i]) or now)
  if tat < now then tat = now end
  local new_tat = tat + interval
  local allow_at = new_tat - interval * burst
  if now < allow_at then
    wait = math.max(wait, allow_at - now)
  end
  tats[i] = new_tat
end
if wait > 0 then
  return {0, math.ceil(wait)}
end
for i = 1, n do
  redis.call('SET', KEYS[i], tostring(tats[i]), 'PX', math.ceil(tats[i] - now) + 1)
end
return {1, 0}
`)

type bucket struct {
	key   string
	rate  float64
	burst float64
}

func (t *Throttle) take(ctx context.Context, buckets []bucket, shareKey string) time.Duration {
	keys := make([]string, 0, len(buckets)+1)
	args := make([]any, 0, 2*len(buckets)+2)
	args = append(args, len(buckets))
	for _, b := range buckets {
		keys = append(keys, b.key)
		args = append(args, strconv.FormatFloat(math.Max(b.rate, 0.01), 'f', 3, 64), strconv.FormatFloat(math.Max(b.burst, 1), 'f', 3, 64))
	}
	if shareKey != "" {
		keys = append(keys, shareKey)
		args = append(args, 1)
	} else {
		args = append(args, 0)
	}
	res, err := sendGCRA.Run(ctx, t.rdb, keys, args...).Int64Slice()
	if err != nil || len(res) != 2 || res[0] == 1 {
		// Redis unavailable: send anyway; Meta's own limit still applies
		// and is handled as a retryable error.
		return 0
	}
	return time.Duration(res[1]) * time.Millisecond
}

// Wait reports how long to wait before the next inbox, API or automation
// send from pn (0 = now).
func (t *Throttle) Wait(ctx context.Context, pn uuid.UUID) time.Duration {
	r := float64(t.Rate(ctx, pn))
	return t.take(ctx, []bucket{{t.numberKey(ctx, pn), r, r}}, "")
}

// WaitCampaign reports how long a send of campaign must wait (0 = now): it
// needs the number's bucket, the campaign lane, the tenant's campaign
// bucket (when capped) and its own share.
func (t *Throttle) WaitCampaign(ctx context.Context, pn, campaign uuid.UUID) time.Duration {
	r := float64(t.Rate(ctx, pn))
	lane := r * (1 - t.reserved)
	buckets := []bucket{{t.numberKey(ctx, pn), r, r}, {t.laneKey(ctx, pn), lane, lane}}
	if t.tenantCap != nil {
		if c := t.tenantCap(ctx); c > 0 {
			buckets = append(buckets, bucket{t.tenantKey(ctx), c, c})
		}
	}
	// The campaign's own bucket comes last: the script replaces its rate
	// with the stored share.
	buckets = append(buckets, bucket{t.campaignKey(ctx, pn, campaign), lane, lane})
	return t.take(ctx, buckets, t.shareKey(ctx, pn, campaign))
}
