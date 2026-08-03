package querybroker

import "testing"

func TestQuotaAllowsUpToTheLimit(t *testing.T) {
	q := NewQuotaTracker(Quota{RequestsPerMinute: 2})
	if !q.Allow("alice") {
		t.Fatal("1st request should be allowed")
	}
	if !q.Allow("alice") {
		t.Fatal("2nd request should be allowed")
	}
	if q.Allow("alice") {
		t.Fatal("3rd request should be denied")
	}
}

func TestQuotaIsPerUser(t *testing.T) {
	q := NewQuotaTracker(Quota{RequestsPerMinute: 1})
	if !q.Allow("alice") {
		t.Fatal("alice's 1st request should be allowed")
	}
	if !q.Allow("bob") {
		t.Fatal("bob's own quota should be independent of alice's")
	}
}

func TestZeroRequestsPerMinuteMeansUnlimited(t *testing.T) {
	q := NewQuotaTracker(Quota{RequestsPerMinute: 0})
	for i := 0; i < 1000; i++ {
		if !q.Allow("alice") {
			t.Fatalf("request %d should be allowed under an unlimited quota", i)
		}
	}
}
