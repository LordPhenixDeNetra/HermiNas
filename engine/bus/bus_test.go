package bus

import (
	"sync"
	"testing"
	"time"
)

func TestPublishDeliversToSubscriber(t *testing.T) {
	b := New(4)
	ch, unsub := b.Subscribe(TopicSystemHealth)
	defer unsub()

	b.Publish(TopicSystemHealth, "clickhouse-healthy")

	select {
	case got := <-ch:
		if got != "clickhouse-healthy" {
			t.Fatalf("unexpected event: %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published event")
	}
}

func TestPublishFansOutToMultipleSubscribers(t *testing.T) {
	b := New(4)
	ch1, unsub1 := b.Subscribe(TopicAlertFired)
	defer unsub1()
	ch2, unsub2 := b.Subscribe(TopicAlertFired)
	defer unsub2()

	b.Publish(TopicAlertFired, "fraud-alert")

	for i, ch := range []<-chan any{ch1, ch2} {
		select {
		case got := <-ch:
			if got != "fraud-alert" {
				t.Fatalf("subscriber %d: unexpected event: %v", i, got)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out waiting for event", i)
		}
	}
}

func TestPublishDoesNotCrossTopics(t *testing.T) {
	b := New(4)
	health, unsubHealth := b.Subscribe(TopicSystemHealth)
	defer unsubHealth()
	alerts, unsubAlerts := b.Subscribe(TopicAlertFired)
	defer unsubAlerts()

	b.Publish(TopicSystemHealth, "ok")

	select {
	case <-alerts:
		t.Fatal("alert.fired subscriber should not receive a system.health event")
	case <-time.After(50 * time.Millisecond):
	}

	select {
	case <-health:
	default:
		t.Fatal("system.health subscriber should have received the event")
	}
}

func TestUnsubscribeStopsDeliveryAndClosesChannel(t *testing.T) {
	b := New(4)
	ch, unsub := b.Subscribe(TopicPipelineStatus)

	unsub()

	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed after Unsubscribe")
	}

	// Publishing after everyone unsubscribed must not panic or block.
	b.Publish(TopicPipelineStatus, "running")

	if n := b.SubscriberCount(TopicPipelineStatus); n != 0 {
		t.Fatalf("expected 0 subscribers after unsubscribe, got %d", n)
	}
}

func TestUnsubscribeIsSafeToCallTwice(t *testing.T) {
	b := New(4)
	_, unsub := b.Subscribe(TopicAgentStatus)
	unsub()
	unsub() // must not panic (double close)
}

func TestSlowSubscriberDropsInsteadOfBlockingPublisher(t *testing.T) {
	b := New(1) // buffer of 1: the second publish must overflow
	ch, unsub := b.Subscribe(TopicAnomalyDetected)
	defer unsub()

	b.Publish(TopicAnomalyDetected, "first") // fills the buffer
	done := make(chan struct{})
	go func() {
		b.Publish(TopicAnomalyDetected, "second") // must not block since nobody's draining
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a full subscriber buffer")
	}

	if got := b.Dropped(TopicAnomalyDetected); got != 1 {
		t.Fatalf("expected 1 dropped event, got %d", got)
	}

	// The first event is still readable — only the second was dropped.
	select {
	case got := <-ch:
		if got != "first" {
			t.Fatalf("unexpected surviving event: %v", got)
		}
	default:
		t.Fatal("expected the first event to still be buffered")
	}
}

func TestConcurrentPublishAndSubscribeIsRaceFree(t *testing.T) {
	b := New(8)
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, unsub := b.Subscribe(TopicDashboardData)
			defer unsub()
			select {
			case <-ch:
			case <-time.After(100 * time.Millisecond):
			}
		}()
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Publish(TopicDashboardData, "tick")
		}()
	}

	wg.Wait()
}
