package messaging

import (
	"math"
	"testing"

	"github.com/google/uuid"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestFairSharesByWeight(t *testing.T) {
	hi, lo, mid := uuid.New(), uuid.New(), uuid.New()
	got := FairShares(64, []Contender{{ID: hi, Weight: WeightOf("high")}, {ID: lo, Weight: WeightOf("low")}})
	if !near(got[hi], 64*4.0/5) || !near(got[lo], 64*1.0/5) {
		t.Fatalf("high/low: %v", got)
	}
	got = FairShares(70, []Contender{{ID: hi, Weight: 4}, {ID: mid, Weight: 2}, {ID: lo, Weight: 1}})
	if !near(got[hi], 40) || !near(got[mid], 20) || !near(got[lo], 10) {
		t.Fatalf("three: %v", got)
	}
	// No starvation: a low-priority campaign always gets its weight's part.
	many := []Contender{{ID: lo, Weight: 1}}
	for range 10 {
		many = append(many, Contender{ID: uuid.New(), Weight: 4})
	}
	if got := FairShares(82, many); !near(got[lo], 2) {
		t.Fatalf("low among ten high: %v", got[lo])
	}
	// Unknown priorities count as normal; zero weights as 1.
	if WeightOf("urgent") != 2 {
		t.Fatal("unknown priority weight")
	}
	if got := FairShares(10, []Contender{{ID: hi}}); !near(got[hi], 10) {
		t.Fatalf("single: %v", got)
	}
}

func TestFairSharesRedistributesCaps(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	// a would get 40 of 60 but is capped at 5: b and c split the other 55
	// by weight.
	got := FairShares(60, []Contender{{ID: a, Weight: 4, Cap: 5}, {ID: b, Weight: 2}, {ID: c, Weight: 1}})
	if !near(got[a], 5) || !near(got[b], 55*2.0/3) || !near(got[c], 55*1.0/3) {
		t.Fatalf("capped: %v", got)
	}
	// A cap above the fair share changes nothing.
	got = FairShares(30, []Contender{{ID: a, Weight: 1, Cap: 100}, {ID: b, Weight: 1}})
	if !near(got[a], 15) || !near(got[b], 15) {
		t.Fatalf("loose cap: %v", got)
	}
	// Everyone capped: capacity is left unused, never exceeded.
	got = FairShares(100, []Contender{{ID: a, Weight: 1, Cap: 3}, {ID: b, Weight: 1, Cap: 4}})
	if !near(got[a], 3) || !near(got[b], 4) {
		t.Fatalf("all capped: %v", got)
	}
	var total float64
	for _, v := range FairShares(50, []Contender{{ID: a, Weight: 4, Cap: 10}, {ID: b, Weight: 2, Cap: 30}, {ID: c, Weight: 1}}) {
		total += v
	}
	if total > 50+1e-9 {
		t.Fatalf("shares exceed capacity: %v", total)
	}
	if got := FairShares(10, nil); len(got) != 0 {
		t.Fatal("no contenders")
	}
}
