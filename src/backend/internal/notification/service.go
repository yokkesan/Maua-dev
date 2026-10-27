package notification

import (
	"context"
	"errors"
	"time"
)

// A failed delivery is known not to have been accepted. All unclassified errors
// are ambiguous and require inspection rather than automatic retransmission.
var ErrNotAccepted = errors.New("delivery not accepted")

type Mailer interface {
	Send(context.Context, Message) error
}
type Service struct {
	Repository *Repository
	Mailer     Mailer
	Config     Config
}
type Event struct {
	ID        int64      `json:"id"`
	Key       string     `json:"event_key"`
	Recipient string     `json:"recipient"`
	Subject   string     `json:"subject"`
	State     string     `json:"state"`
	Attempts  int        `json:"attempts"`
	LastError *string    `json:"last_error"`
	CreatedAt time.Time  `json:"created_at"`
	SentAt    *time.Time `json:"sent_at"`
}

func (s *Service) List(ctx context.Context, before int64) ([]Event, error) {
	return s.Repository.List(ctx, before)
}
func (s *Service) DispatchOne(ctx context.Context) (bool, error) {
	if s.Mailer == nil {
		return false, errors.New("notification mailer is required")
	}
	event, err := s.Repository.claim(ctx)
	if err != nil {
		return false, err
	}
	if event == nil {
		return false, nil
	}
	m := event.Message
	if event.Encrypted {
		m.Body, err = open(s.Config.EncryptionKey, m.Body)
	}
	state, code := "sent", ""
	if err != nil {
		state, code = "failed", "payload_unavailable"
	} else {
		deliveryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = s.Mailer.Send(deliveryCtx, m)
		cancel()
		if err != nil {
			state, code = "unknown", "delivery_unknown"
			if errors.Is(err, ErrNotAccepted) {
				state, code = "failed", "delivery_not_accepted"
			}
		}
	}
	// A cancelled HTTP/CLI context must not discard an already observed result.
	cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saveErr := s.Repository.finish(cleanup, event, state, code)
	return true, saveErr
}
