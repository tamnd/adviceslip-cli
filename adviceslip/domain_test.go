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

func TestClassifyNumeric(t *testing.T) {
	typ, id, err := Domain{}.Classify("42")
	if err != nil {
		t.Fatal(err)
	}
	if typ != "id" {
		t.Errorf("Classify(42) type = %q, want id", typ)
	}
	if id != "42" {
		t.Errorf("Classify(42) id = %q, want 42", id)
	}
}

func TestClassifyText(t *testing.T) {
	typ, id, err := Domain{}.Classify("money")
	if err != nil {
		t.Fatal(err)
	}
	if typ != "query" {
		t.Errorf("Classify(money) type = %q, want query", typ)
	}
	if id != "money" {
		t.Errorf("Classify(money) id = %q, want money", id)
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify empty string should return error")
	}
}

func TestLocateID(t *testing.T) {
	got, err := Domain{}.Locate("id", "42")
	want := "https://" + Host + "/advice/42"
	if err != nil || got != want {
		t.Errorf("Locate(id,42) = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateQuery(t *testing.T) {
	got, err := Domain{}.Locate("query", "money")
	want := "https://" + Host + "/advice/search/money"
	if err != nil || got != want {
		t.Errorf("Locate(query,money) = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "42")
	if err == nil {
		t.Error("Locate with unknown type should return error")
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	got, err := h.ResolveOn("adviceslip", "42")
	if err != nil || got.String() != "adviceslip://id/42" {
		t.Errorf("ResolveOn = (%q, %v), want adviceslip://id/42", got.String(), err)
	}
}
