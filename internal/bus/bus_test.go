package bus

import "testing"

func TestPublishTracksDropsAndSubscribers(t *testing.T) {
	b := New()
	sub, _ := b.Subscribe()
	if got := b.SubscriberCount(); got != 1 {
		t.Fatalf("SubscriberCount = %d, want 1", got)
	}
	if got := b.Dropped(); got != 0 {
		t.Fatalf("Dropped initially = %d", got)
	}

	// The subscriber channel holds 256 events. Publishing 300 must drop the
	// overflow rather than block the organism.
	for i := 0; i < 300; i++ {
		b.Publish("test", map[string]int{"i": i})
	}
	if got := b.Dropped(); got == 0 {
		t.Fatal("slow subscriber dropped no events")
	}

	b.Unsubscribe(sub)
	if got := b.SubscriberCount(); got != 0 {
		t.Fatalf("SubscriberCount after unsubscribe = %d", got)
	}
}
