package adviceslip

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring, which need no network. The client's HTTP behaviour is
// covered in adviceslip_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "adviceslip" {
		t.Errorf("Scheme = %q, want adviceslip", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "adviceslip" {
		t.Errorf("Identity.Binary = %q, want adviceslip", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"42", "slip", "42"},
		{"life", "slip", "life"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("slip", "42")
	want := "https://" + Host + "/advice/42"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	got, err := h.ResolveOn("adviceslip", "42")
	if err != nil || got.String() != "adviceslip://slip/42" {
		t.Errorf("ResolveOn = (%q, %v), want adviceslip://slip/42", got.String(), err)
	}
}
