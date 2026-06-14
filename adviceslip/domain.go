package adviceslip

import (
	"context"
	"fmt"
	"strings"

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

Get a random piece of advice or search for slips by keyword.`,
			Site: Host,
			Repo: "https://github.com/tamnd/adviceslip-cli",
		},
	}
}

// Register installs the client factory and all operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "random", Group: "read", Single: true,
		Summary: "Get a random piece of advice"}, randomOp)

	kit.Handle(app, kit.OpMeta{Name: "search", Group: "read", List: true,
		Summary: "Search advice slips by keyword",
		Args:    []kit.Arg{{Name: "query", Help: "search query"}}}, searchOp)
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

// --- input structs ---

type randomInput struct {
	Client *Client `kit:"inject"`
}

type searchInput struct {
	Query  string  `kit:"arg" help:"search query"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func randomOp(ctx context.Context, in randomInput, emit func(*Slip) error) error {
	slip, err := in.Client.Random(ctx)
	if err != nil {
		return err
	}
	return emit(slip)
}

func searchOp(ctx context.Context, in searchInput, emit func(Slip) error) error {
	slips, err := in.Client.Search(ctx, in.Query, in.Limit)
	if err != nil {
		return err
	}
	for _, s := range slips {
		if err := emit(s); err != nil {
			return err
		}
	}
	return nil
}

// Classify turns a slip id into (type, id).
func (Domain) Classify(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty adviceslip reference")
	}
	return "slip", input, nil
}

// Locate returns the live API URL for a (type, id).
func (Domain) Locate(t, id string) (string, error) {
	if t != "slip" {
		return "", errs.Usage("adviceslip has no resource type %q", t)
	}
	return fmt.Sprintf("%s/advice/%s", BaseURL, id), nil
}
