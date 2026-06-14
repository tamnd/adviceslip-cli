package adviceslip

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the adviceslip kit driver. It carries no state; the per-run
// client is built by the factory Register hands to kit.
type Domain struct{}

// Info describes the scheme, hostnames, and the identity used in help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "adviceslip",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "adviceslip",
			Short:  "A command line for the Advice Slip API.",
			Long: `A command line for the Advice Slip API.

adviceslip reads public advice data from api.adviceslip.com over plain HTTPS,
shapes it into clean records, and prints output that pipes into the rest of your
tools. No API key, nothing to run alongside it.

Get a random piece of advice, look up a slip by ID, or search by keyword.`,
			Site: Host,
			Repo: "https://github.com/tamnd/adviceslip-cli",
		},
	}
}

// Register installs the client factory and all operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "random", Group: "read", Single: true,
		Summary: "Get a random piece of advice"}, adviceOp)

	kit.Handle(app, kit.OpMeta{Name: "get", Group: "read", Single: true,
		Summary: "Get an advice slip by ID",
		Args:    []kit.Arg{{Name: "id", Help: "slip ID number"}}}, getOp)

	kit.Handle(app, kit.OpMeta{Name: "search", Group: "read", List: true,
		Summary: "Search advice slips by keyword",
		Args:    []kit.Arg{{Name: "query", Help: "search term"}}}, searchOp)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// Advice is the output record for all three operations.
type Advice struct {
	ID     int    `kit:"id" json:"id"`
	Advice string `json:"advice"`
	Query  string `json:"query,omitempty"`
}

// --- input structs ---

type adviceInput struct {
	Client *Client `kit:"inject"`
}

type getInput struct {
	ID     int     `kit:"arg" help:"slip ID number"`
	Client *Client `kit:"inject"`
}

type searchInput struct {
	Query  string  `kit:"arg" help:"search term"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func adviceOp(ctx context.Context, in adviceInput, emit func(*Advice) error) error {
	slip, err := in.Client.Random(ctx)
	if err != nil {
		return err
	}
	return emit(&Advice{ID: slip.ID, Advice: slip.Advice})
}

func getOp(ctx context.Context, in getInput, emit func(*Advice) error) error {
	slip, err := in.Client.Get(ctx, in.ID)
	if err != nil {
		return err
	}
	return emit(&Advice{ID: slip.ID, Advice: slip.Advice})
}

func searchOp(ctx context.Context, in searchInput, emit func(*Advice) error) error {
	slips, err := in.Client.Search(ctx, in.Query, 0)
	if err != nil {
		return err
	}
	for _, s := range slips {
		if err := emit(&Advice{ID: s.ID, Advice: s.Advice, Query: in.Query}); err != nil {
			return err
		}
	}
	return nil
}

// isNumeric returns true when every rune in s is a decimal digit.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// Classify turns a reference into (type, id).
// Numeric strings become ("id", input); everything else becomes ("query", input).
func (Domain) Classify(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty adviceslip reference")
	}
	if isNumeric(input) {
		return "id", input, nil
	}
	return "query", input, nil
}

// Locate returns the live API URL for a (type, id) pair.
func (Domain) Locate(t, id string) (string, error) {
	switch t {
	case "id":
		if _, err := strconv.Atoi(id); err != nil {
			return "", errs.Usage("id %q is not a valid integer", id)
		}
		return fmt.Sprintf("%s/advice/%s", BaseURL, id), nil
	case "query":
		return fmt.Sprintf("%s/advice/search/%s", BaseURL, id), nil
	default:
		return "", errs.Usage("adviceslip has no resource type %q", t)
	}
}
