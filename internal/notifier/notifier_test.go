package notifier

import (
	"testing"
	"time"
)

// helper — ceka dogadjaj sa timeoutom
func waitEvent(ch <-chan Event, timeout time.Duration) (Event, bool) {
	select {
	case e := <-ch:
		return e, true
	case <-time.After(timeout):
		return Event{}, false
	}
}

// ============================================================
// 1. Osnovni subscribe i notify
// ============================================================

func TestSubscribe_ReceivesPutInRange(t *testing.T) {
	n := NewNotifier()
	l := n.Subscribe([]byte("a"), []byte("c"), 16)

	n.NotifyPut([]byte("b"), []byte("hello"))

	e, ok := waitEvent(l.Ch, time.Second)
	if !ok {
		t.Fatal("expected event, got timeout")
	}
	if e.Type != EventPut {
		t.Errorf("type = %v, want PUT", e.Type)
	}
	if string(e.Key) != "b" {
		t.Errorf("key = %q, want b", e.Key)
	}
	if string(e.Value) != "hello" {
		t.Errorf("value = %q, want hello", e.Value)
	}
}

func TestSubscribe_ReceivesDeleteInRange(t *testing.T) {
	n := NewNotifier()
	l := n.Subscribe([]byte("a"), []byte("c"), 16)

	n.NotifyDelete([]byte("b"))

	e, ok := waitEvent(l.Ch, time.Second)
	if !ok {
		t.Fatal("expected event, got timeout")
	}
	if e.Type != EventDelete {
		t.Errorf("type = %v, want DELETE", e.Type)
	}
	if e.Value != nil {
		t.Errorf("value should be nil for delete, got %v", e.Value)
	}
}

// ============================================================
// 2. Kljuc izvan opsega — ne smije stici
// ============================================================

func TestSubscribe_NoEventOutOfRange(t *testing.T) {
	n := NewNotifier()
	l := n.Subscribe([]byte("a"), []byte("c"), 16)

	n.NotifyPut([]byte("d"), []byte("value")) // izvan opsega
	n.NotifyPut([]byte("z"), []byte("value")) // izvan opsega

	_, ok := waitEvent(l.Ch, 50*time.Millisecond)
	if ok {
		t.Error("should not receive event for key outside range")
	}
}

// ============================================================
// 3. Granicni kljucevi — lower i upper su ukljuceni
// ============================================================

func TestSubscribe_LowerBoundInclusive(t *testing.T) {
	n := NewNotifier()
	l := n.Subscribe([]byte("apple"), []byte("cherry"), 16)

	n.NotifyPut([]byte("apple"), []byte("v"))

	_, ok := waitEvent(l.Ch, time.Second)
	if !ok {
		t.Error("lower bound key should trigger event")
	}
}

func TestSubscribe_UpperBoundInclusive(t *testing.T) {
	n := NewNotifier()
	l := n.Subscribe([]byte("apple"), []byte("cherry"), 16)

	n.NotifyPut([]byte("cherry"), []byte("v"))

	_, ok := waitEvent(l.Ch, time.Second)
	if !ok {
		t.Error("upper bound key should trigger event")
	}
}

func TestSubscribe_JustBelowLower_NoEvent(t *testing.T) {
	n := NewNotifier()
	l := n.Subscribe([]byte("b"), []byte("d"), 16)

	n.NotifyPut([]byte("a"), []byte("v"))

	_, ok := waitEvent(l.Ch, 50*time.Millisecond)
	if ok {
		t.Error("key just below lower bound should not trigger event")
	}
}

func TestSubscribe_JustAboveUpper_NoEvent(t *testing.T) {
	n := NewNotifier()
	l := n.Subscribe([]byte("b"), []byte("d"), 16)

	n.NotifyPut([]byte("e"), []byte("v"))

	_, ok := waitEvent(l.Ch, 50*time.Millisecond)
	if ok {
		t.Error("key just above upper bound should not trigger event")
	}
}

// ============================================================
// 4. Vise slusaoca istovremeno
// ============================================================

func TestMultipleListeners_CorrectRouting(t *testing.T) {
	n := NewNotifier()

	// l1 slusa [a, c], l2 slusa [d, f]
	l1 := n.Subscribe([]byte("a"), []byte("c"), 16)
	l2 := n.Subscribe([]byte("d"), []byte("f"), 16)

	n.NotifyPut([]byte("b"), []byte("v1")) // samo l1
	n.NotifyPut([]byte("e"), []byte("v2")) // samo l2

	// l1 treba da dobije "b"
	e1, ok1 := waitEvent(l1.Ch, time.Second)
	if !ok1 || string(e1.Key) != "b" {
		t.Errorf("l1: expected key=b, got %v ok=%v", e1.Key, ok1)
	}

	// l2 treba da dobije "e"
	e2, ok2 := waitEvent(l2.Ch, time.Second)
	if !ok2 || string(e2.Key) != "e" {
		t.Errorf("l2: expected key=e, got %v ok=%v", e2.Key, ok2)
	}

	// l1 ne smije imati "e"
	_, extra := waitEvent(l1.Ch, 50*time.Millisecond)
	if extra {
		t.Error("l1 should not receive event for key=e")
	}
}

func TestMultipleListeners_OverlappingRanges(t *testing.T) {
	n := NewNotifier()

	// Oba slusaju [a, z] — oba trebaju dobiti isti dogadjaj
	l1 := n.Subscribe([]byte("a"), []byte("z"), 16)
	l2 := n.Subscribe([]byte("a"), []byte("z"), 16)

	n.NotifyPut([]byte("m"), []byte("v"))

	_, ok1 := waitEvent(l1.Ch, time.Second)
	_, ok2 := waitEvent(l2.Ch, time.Second)

	if !ok1 {
		t.Error("l1 should receive event")
	}
	if !ok2 {
		t.Error("l2 should receive event")
	}
}

// ============================================================
// 5. Unsubscribe
// ============================================================

func TestUnsubscribe_StopsEvents(t *testing.T) {
	n := NewNotifier()
	l := n.Subscribe([]byte("a"), []byte("z"), 16)

	n.Unsubscribe(l)

	// kanal je zatvoren — range ce odmah izaci
	count := 0
	for range l.Ch {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 events after unsubscribe, got %d", count)
	}
}

func TestUnsubscribe_OtherListenersStillWork(t *testing.T) {
	n := NewNotifier()
	l1 := n.Subscribe([]byte("a"), []byte("z"), 16)
	l2 := n.Subscribe([]byte("a"), []byte("z"), 16)

	n.Unsubscribe(l1)
	n.NotifyPut([]byte("b"), []byte("v"))

	// l2 treba da dobije dogadjaj
	_, ok := waitEvent(l2.Ch, time.Second)
	if !ok {
		t.Error("l2 should still receive events after l1 unsubscribed")
	}
}

func TestUnsubscribe_Twice_NoPanic(t *testing.T) {
	n := NewNotifier()
	l := n.Subscribe([]byte("a"), []byte("z"), 16)

	// dupli unsubscribe ne smije panic-ovati
	n.Unsubscribe(l)
	n.Unsubscribe(l)
}

// ============================================================
// 6. Vise dogadjaja u nizu
// ============================================================

func TestMultipleEvents_Order(t *testing.T) {
	n := NewNotifier()
	l := n.Subscribe([]byte("a"), []byte("z"), 16)

	keys := []string{"b", "c", "d", "e"}
	for _, k := range keys {
		n.NotifyPut([]byte(k), []byte("v"))
	}

	for _, want := range keys {
		e, ok := waitEvent(l.Ch, time.Second)
		if !ok {
			t.Fatalf("expected event for key=%s, got timeout", want)
		}
		if string(e.Key) != want {
			t.Errorf("got key=%q, want %q", e.Key, want)
		}
	}
}

func TestMixedPutDelete_BothReceived(t *testing.T) {
	n := NewNotifier()
	l := n.Subscribe([]byte("a"), []byte("z"), 16)

	n.NotifyPut([]byte("b"), []byte("v"))
	n.NotifyDelete([]byte("b"))

	e1, _ := waitEvent(l.Ch, time.Second)
	e2, _ := waitEvent(l.Ch, time.Second)

	if e1.Type != EventPut {
		t.Errorf("first event: got %v, want PUT", e1.Type)
	}
	if e2.Type != EventDelete {
		t.Errorf("second event: got %v, want DELETE", e2.Type)
	}
}

// ============================================================
// 7. Goroutine — notify iz vise gorutina istovremeno
// ============================================================

func TestConcurrentNotify(t *testing.T) {
	n := NewNotifier()
	l := n.Subscribe([]byte("a"), []byte("z"), 256)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			n.NotifyPut([]byte("b"), []byte("v"))
		}
		close(done)
	}()
	<-done

	// daj malo vremena da se kanal popuni
	time.Sleep(10 * time.Millisecond)

	count := 0
	for {
		_, ok := waitEvent(l.Ch, 10*time.Millisecond)
		if !ok {
			break
		}
		count++
	}

	if count == 0 {
		t.Error("expected at least some events from concurrent notify")
	}
}

func TestConcurrentSubscribeUnsubscribe_NoPanic(t *testing.T) {
	n := NewNotifier()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			n.NotifyPut([]byte("b"), []byte("v"))
		}
		close(done)
	}()

	for i := 0; i < 10; i++ {
		l := n.Subscribe([]byte("a"), []byte("z"), 8)
		go n.Unsubscribe(l)
	}

	<-done
}
