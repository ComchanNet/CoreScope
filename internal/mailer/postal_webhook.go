package mailer

import (
	"encoding/json"
	"errors"
	"strconv"
)

type postalWebhookMessage struct {
	ID int64  `json:"id"`
	To string `json:"to"`
}

// postalWebhook is the body Postal POSTs to a webhook: one event per request.
type postalWebhook struct {
	Event     string      `json:"event"`
	Timestamp postalFloat `json:"timestamp"`
	Payload   struct {
		Message         *postalWebhookMessage `json:"message"`
		OriginalMessage *postalWebhookMessage `json:"original_message"` // MessageBounced
		Details         string                `json:"details"`
		Timestamp       postalFloat           `json:"timestamp"`
	} `json:"payload"`
}

// postalWebhookEvents maps the message webhook events to canonical names.
// Server events (DomainDNSError, SendLimit*) are not about a message.
var postalWebhookEvents = map[string]string{
	"MessageSent": EventDelivered, "MessageDelayed": EventDeferred,
	"MessageDeliveryFailed": EventHardBounce, "MessageHeld": EventBlocked,
	"MessageBounced": EventHardBounce,
	"MessageLoaded":  EventOpened, "MessageLinkClicked": EventClicked,
}

// ParsePostalWebhook parses one Postal webhook body. It returns an error for
// events that are not about a message, or that carry no message id.
func ParsePostalWebhook(body []byte) ([]Event, error) {
	var w postalWebhook
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, err
	}
	event, ok := postalWebhookEvents[w.Event]
	if !ok {
		return nil, errors.New("postal webhook: not a message event")
	}
	msg := w.Payload.Message
	if msg == nil {
		msg = w.Payload.OriginalMessage
	}
	if msg == nil || msg.ID <= 0 {
		return nil, errors.New("postal webhook: no message id")
	}
	at := w.Payload.Timestamp.time()
	if at.IsZero() {
		at = w.Timestamp.timeOrNow()
	}
	return []Event{{MessageID: strconv.FormatInt(msg.ID, 10), Email: msg.To, Event: event,
		Reason: w.Payload.Details, At: at}}, nil
}
