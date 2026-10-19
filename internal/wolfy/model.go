package wolfy

import (
	"context"
	"time"
)

const Retention = 7 * 24 * time.Hour

// Job states are durable boundaries around external side effects.
const (
	Pending   = "pending"
	Preparing = "preparing"
	Ready     = "ready"
	Sending   = "sending"
	Sent      = "sent"
	Uncertain = "uncertain"
	Failed    = "failed"
	Ignored   = "ignored"
)

type Job struct {
	Sequence     uint64    `json:"sequence"`
	ID           string    `json:"id"`
	User         string    `json:"user,omitempty"`
	Text         string    `json:"text,omitempty"`
	Kind         string    `json:"kind"`
	Received     time.Time `json:"received"`
	MessageTime  time.Time `json:"message_time"`
	State        string    `json:"state"`
	Due          time.Time `json:"due"`
	Attempts     int       `json:"attempts"`
	SendAttempts int       `json:"send_attempts"`
	Reply        string    `json:"reply,omitempty"`
	ReplyAt      time.Time `json:"reply_at"`
	Remember     bool      `json:"remember"`
	ProviderID   string    `json:"provider_id,omitempty"`
}

type Turn struct {
	Role string    `json:"role"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

type Event struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Date     string `json:"date"`
	Time     string `json:"time"`
	VenueID  string `json:"venue_id"`
	Opponent string `json:"opponent,omitempty"`
	Status   string `json:"status"`
	Notes    string `json:"notes,omitempty"`
	Updated  string `json:"updated_at"`
}

type Venue struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

type FAQ struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Updated  string `json:"updated_at"`
}

// Knowledge is deliberately separate from the authorization list.
type Knowledge struct {
	Events  []Event `json:"events"`
	Venues  []Venue `json:"venues"`
	FAQ     []FAQ   `json:"faq"`
	Contact string  `json:"contact"`
}

type Directory interface {
	Authorized(context.Context, string) (bool, error)
	Knowledge(context.Context) (Knowledge, error)
}

type ModelRequest struct {
	Instructions string
	Input        string
	MaxOutput    int
}

type ModelResult struct {
	Text         string
	InputTokens  int64
	OutputTokens int64
	UsageKnown   bool
}

type Model interface {
	Generate(context.Context, ModelRequest) (ModelResult, error)
}

type Delivery struct {
	ID string
	// Retry is true ONLY when the provider explicitly rejected the send temporarily.
	Retry bool
	// Certain means that the request is known to have been rejected, not accepted.
	Certain bool
}

type Messenger interface {
	Send(context.Context, string, string) (Delivery, error)
}
