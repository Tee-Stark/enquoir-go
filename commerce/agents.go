package commerce

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

// MerchantAgent is an AI agent the merchant runs - a shopping assistant,
// say. Named here MerchantAgent, not Agent, because treasury's API-key
// identity is also called an agent; the two are unrelated, and this SDK
// keeps them from colliding. It holds no credential of its own - the
// merchant's own backend names it in checkout requests.
type MerchantAgent struct {
	ID         string
	ExternalID string
	Name       string
	Status     CommerceStatus
	CreatedAt  time.Time
	DisabledAt *time.Time
}

type merchantAgentWire struct {
	ID         string  `json:"id"`
	ExternalID string  `json:"external_id"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
	DisabledAt *string `json:"disabled_at"`
}

func (w merchantAgentWire) toMerchantAgent() (*MerchantAgent, error) {
	createdAt, err := parseTime(w.CreatedAt)
	if err != nil {
		return nil, err
	}
	disabledAt, err := parseTimePtr(w.DisabledAt)
	if err != nil {
		return nil, err
	}
	return &MerchantAgent{
		ID:         w.ID,
		ExternalID: w.ExternalID,
		Name:       w.Name,
		Status:     CommerceStatus(w.Status),
		CreatedAt:  createdAt,
		DisabledAt: disabledAt,
	}, nil
}

// MerchantAgents registers and manages the merchant's AI agents.
type MerchantAgents struct {
	transport *transport.Client
}

// NewMerchantAgents builds a MerchantAgents resource over an existing
// transport. Callers use enquoir.NewMerchantClient instead of this
// directly.
func NewMerchantAgents(t *transport.Client) *MerchantAgents {
	return &MerchantAgents{transport: t}
}

// RegisterMerchantAgentRequest identifies an agent to register.
type RegisterMerchantAgentRequest struct {
	ExternalID string
	Name       string
}

// Register registers an agent, or resolves one already registered with
// the same external ID and name.
func (a *MerchantAgents) Register(ctx context.Context, req RegisterMerchantAgentRequest) (*MerchantAgent, error) {
	wire := struct {
		ExternalID string `json:"external_id"`
		Name       string `json:"name"`
	}{ExternalID: req.ExternalID, Name: req.Name}

	var out merchantAgentWire
	if err := a.transport.Do(ctx, http.MethodPost, "/v1/commerce/agents", wire, &out); err != nil {
		return nil, err
	}
	return out.toMerchantAgent()
}

// Get reads one agent by Enquoir's ID.
func (a *MerchantAgents) Get(ctx context.Context, id string) (*MerchantAgent, error) {
	var out merchantAgentWire
	path := "/v1/commerce/agents/" + url.PathEscape(id)
	if err := a.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toMerchantAgent()
}

// Disable disables an agent. Disabling an already-disabled agent
// succeeds and changes nothing.
func (a *MerchantAgents) Disable(ctx context.Context, id string) (*MerchantAgent, error) {
	var out merchantAgentWire
	path := "/v1/commerce/agents/" + url.PathEscape(id) + "/disable"
	if err := a.transport.Do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toMerchantAgent()
}

// Enable enables an agent.
func (a *MerchantAgents) Enable(ctx context.Context, id string) (*MerchantAgent, error) {
	var out merchantAgentWire
	path := "/v1/commerce/agents/" + url.PathEscape(id) + "/enable"
	if err := a.transport.Do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toMerchantAgent()
}

// MerchantAgentListOptions filters and pages List. Zero values mean
// "no filter" or "use the server's default."
type MerchantAgentListOptions struct {
	ExternalID string
	Status     CommerceStatus
	Limit      int
	Cursor     string
}

func (o MerchantAgentListOptions) query() url.Values {
	q := url.Values{}
	if o.ExternalID != "" {
		q.Set("external_id", o.ExternalID)
	}
	if o.Status != "" {
		q.Set("status", string(o.Status))
	}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Cursor != "" {
		q.Set("cursor", o.Cursor)
	}
	return q
}

// List lists the merchant's agents, newest first.
func (a *MerchantAgents) List(ctx context.Context, opts MerchantAgentListOptions) (*Page[MerchantAgent], error) {
	path := "/v1/commerce/agents"
	if q := opts.query(); len(q) > 0 {
		path += "?" + q.Encode()
	}

	var out struct {
		Data       []merchantAgentWire `json:"data"`
		NextCursor *string             `json:"next_cursor"`
	}
	if err := a.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}

	page := &Page[MerchantAgent]{NextCursor: stringOrEmpty(out.NextCursor)}
	for _, w := range out.Data {
		agent, err := w.toMerchantAgent()
		if err != nil {
			return nil, err
		}
		page.Data = append(page.Data, *agent)
	}
	return page, nil
}
