package mailer

import (
	"testing"
	"time"
)

func TestParsePostalWebhookDeliveryEvents(t *testing.T) {
	cases := map[string]string{
		"MessageSent": EventDelivered, "MessageDelayed": EventDeferred,
		"MessageDeliveryFailed": EventHardBounce, "MessageHeld": EventBlocked,
	}
	for name, want := range cases {
		body := []byte(`{"event":"` + name + `","timestamp":1791288010.5,"uuid":"u1","payload":{
			"message":{"id":1234,"token":"tk","direction":"outgoing","message_id":"x@rp","to":"a@example.org",
				"from":"noreply@example.org","subject":"S","timestamp":1791288000.0,"tag":"activate"},
			"status":"Sent","details":"Message accepted","output":"250 OK","sent_with_ssl":true,
			"timestamp":1791288005.0,"time":0.2}}`)
		evs, err := ParsePostalWebhook(body)
		if err != nil || len(evs) != 1 {
			t.Fatalf("%s: ParsePostalWebhook = %+v, %v", name, evs, err)
		}
		e := evs[0]
		if e.MessageID != "1234" || e.Email != "a@example.org" || e.Event != want || e.Reason != "Message accepted" ||
			!e.At.Equal(time.Unix(1791288005, 0).UTC()) {
			t.Errorf("%s: event = %+v", name, e)
		}
	}
}

func TestParsePostalWebhookBounceAndTracking(t *testing.T) {
	bounce := []byte(`{"event":"MessageBounced","timestamp":1791288100,"payload":{
		"original_message":{"id":1234,"to":"a@example.org"},
		"bounce":{"id":5678,"to":"noreply@example.org"}}}`)
	evs, err := ParsePostalWebhook(bounce)
	if err != nil || len(evs) != 1 || evs[0].MessageID != "1234" || evs[0].Event != EventHardBounce ||
		!evs[0].At.Equal(time.Unix(1791288100, 0).UTC()) {
		t.Fatalf("bounce = %+v, %v", evs, err)
	}
	for name, want := range map[string]string{"MessageLoaded": EventOpened, "MessageLinkClicked": EventClicked} {
		body := []byte(`{"event":"` + name + `","timestamp":1791288200,"payload":{"message":{"id":1234},"url":"https://x","ip_address":"192.0.2.1"}}`)
		evs, err := ParsePostalWebhook(body)
		if err != nil || len(evs) != 1 || evs[0].Event != want || evs[0].MessageID != "1234" {
			t.Errorf("%s = %+v, %v", name, evs, err)
		}
	}
}

func TestParsePostalWebhookTimestampFallback(t *testing.T) {
	evs, err := ParsePostalWebhook([]byte(`{"event":"MessageSent","payload":{"message":{"id":1}}}`))
	if err != nil || len(evs) != 1 || evs[0].At.IsZero() {
		t.Fatalf("no timestamps = %+v, %v", evs, err)
	}
}

func TestParsePostalWebhookStringTimestamp(t *testing.T) {
	// The delivery timestamp is a decimal column, which Rails encodes as a string.
	evs, err := ParsePostalWebhook([]byte(`{"event":"MessageDeliveryFailed","timestamp":1791288010.5,
		"payload":{"message":{"id":9},"status":"HardFail","details":"550 no such user","timestamp":"1791288005.125"}}`))
	if err != nil || len(evs) != 1 || !evs[0].At.Equal(time.Unix(1791288005, 125e6).UTC()) || evs[0].Event != EventHardBounce {
		t.Fatalf("string timestamp = %+v, %v", evs, err)
	}
	if _, err := ParsePostalWebhook([]byte(`{"event":"MessageSent","payload":{"message":{"id":9},"timestamp":"soon"}}`)); err == nil {
		t.Fatal("non-numeric timestamp accepted")
	}
}

func TestParsePostalWebhookRejectsUnusable(t *testing.T) {
	for _, body := range []string{
		``, `not json`, `{}`, `[]`,
		`{"event":"MessageSent","payload":{}}`,
		`{"event":"MessageSent","payload":{"message":{"id":0}}}`,
		`{"event":"DomainDNSError","payload":{"domain":"example.org"}}`,
		`{"event":"SendLimitApproaching","payload":{"server":{}}}`,
	} {
		if _, err := ParsePostalWebhook([]byte(body)); err == nil {
			t.Errorf("ParsePostalWebhook(%q) accepted", body)
		}
	}
}
