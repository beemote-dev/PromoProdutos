package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const magaluURL = "https://www.magazineluiza.com.br/fogao-cooktop-inducao-philco-pct40a-preto-4-bocas-220v-vidro-ceramico-touch-9-niveis/p/ed32c3cbdk/ed/ck4b/?seller_id=techshop"

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestMagaluRealPixFixture(t *testing.T) {
	u, _ := url.Parse(magaluURL)
	v, s, e := magaluPixPrice(fixture(t, "magalu-pix.html"), u)
	if e != nil || v != 1029.99 || s != "techshop" {
		t.Fatalf("price=%v seller=%s error=%v", v, s, e)
	}
}
func TestMagaluRejectsUnverifiedOffers(t *testing.T) {
	original := fixture(t, "magalu-pix.html")
	cases := map[string]string{
		"wrong product":    strings.ReplaceAll(original, `"id": "ed32c3cbdk"`, `"id": "other"`),
		"wrong seller":     strings.ReplaceAll(original, `"id": "techshop"`, `"id": "other"`),
		"wrong variant":    strings.ReplaceAll(original, `"variationId": "ed32c3cbdk"`, `"variationId": "other"`),
		"USD":              strings.ReplaceAll(original, `"currency": "BRL"`, `"currency": "USD"`),
		"missing currency": strings.ReplaceAll(original, `"currency": "BRL"`, `"currency": ""`),
		"installments":     strings.ReplaceAll(original, `"paymentMethodId": "pix"`, `"paymentMethodId": "mastercard"`),
		"out of stock":     strings.ReplaceAll(original, `"available": true`, `"available": false`),
		"inactive seller":  strings.ReplaceAll(original, `"isActive": true`, `"isActive": false`),
		"zero":             strings.ReplaceAll(original, `1029.99`, `0`),
		"negative":         strings.ReplaceAll(original, `1029.99`, `-1`),
		"fractional cents": strings.ReplaceAll(original, `1029.99`, `1029.999`),
		"overflow":         strings.ReplaceAll(original, `1029.99`, `1e999`),
		"missing pix":      strings.ReplaceAll(original, `"bestPrice"`, `"unused"`),
		"invalid json":     `<script id="__NEXT_DATA__">{broken}</script>`,
		"generic metadata": `<meta property="product:price:amount" content="99.90"><script type="application/ld+json">{"@type":"Product","offers":{"price":99.90,"priceCurrency":"BRL"}}</script>`,
		"duplicate state":  original + original,
	}
	u, _ := url.Parse(magaluURL)
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if v, _, e := magaluPixPrice(body, u); e == nil {
				t.Fatalf("accepted %v", v)
			}
		})
	}
	for _, raw := range []string{strings.ReplaceAll(magaluURL, "seller_id=techshop", ""), "https://www.magazineluiza.com.br/?seller_id=techshop"} {
		u, _ := url.Parse(raw)
		if _, _, e := magaluPixPrice(original, u); e == nil {
			t.Fatal("accepted missing identity")
		}
	}
}
func TestMagaluIgnoresRecommendationsAndFallback(t *testing.T) {
	body := fixture(t, "magalu-pix.html")
	// Decoys outside data.item.offers must never become the cash price.
	body = strings.Replace(body, `"data": {`, `"data": {"recommendations":{"price":1},`, 1)
	body = strings.Replace(body, `"offers": [`, `"itemFallback":{"offers":[{"price":2}]},"offers": [`, 1)
	body = `<div id="recommendations">R$ 3,00</div>` + body
	u, _ := url.Parse(magaluURL)
	v, _, e := magaluPixPrice(body, u)
	if e != nil || v != 1029.99 {
		t.Fatalf("got %v %v", v, e)
	}
	// Raw script JSON must not be HTML-unescaped (quoted descriptions).
	body = strings.Replace(body, `"title":`, `"description":"&quot;quoted&quot;", "title":`, 1)
	if _, _, e := magaluPixPrice(body, u); e != nil {
		t.Fatal(e)
	}
}
func TestMagaluRejectsDuplicateSeller(t *testing.T) {
	body := fixture(t, "magalu-pix.html")
	payload := scriptTagRE.FindStringSubmatch(body)[2]
	var state map[string]any
	if e := json.Unmarshal([]byte(payload), &state); e != nil {
		t.Fatal(e)
	}
	item := state["props"].(map[string]any)["pageProps"].(map[string]any)["data"].(map[string]any)["item"].(map[string]any)
	offers := item["offers"].([]any)
	item["offers"] = append(offers, offers[0])
	raw, _ := json.Marshal(state)
	u, _ := url.Parse(magaluURL)
	if _, _, e := magaluPixPrice(`<script id="__NEXT_DATA__">`+string(raw)+`</script>`, u); e == nil {
		t.Fatal("ambiguous offer accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func useTransport(t *testing.T, f roundTripFunc) {
	t.Helper()
	old := client
	client = &http.Client{Transport: f}
	t.Cleanup(func() { client = old })
}
func response(r *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}
}
func TestFetchDiagnosticsAndNeverReturnBlockedPrice(t *testing.T) {
	good := fixture(t, "magalu-pix.html")
	cases := []struct {
		name, body, code string
		status           int
	}{
		{"ok", good, "ok", 200},
		{"denied", fixture(t, "dako-supreme-blocked.html"), "access_denied", 403},
		{"Amazon", fixture(t, "midea-built-blocked.html"), "remote_error", 503},
		{"captcha200", `<title>Robot Check</title>` + good, "captcha", 200},
		{"denied200", `<title>Access Denied</title>` + good, "access_denied_page", 200},
		{"rate limit", good, "rate_limited", 429},
		{"auth", good, "authentication_required", 401},
		{"not found", good, "http_error", 404},
		{"too large", strings.Repeat("x", maxPageBytes+1), "body_too_large", 200},
		{"no verified data", `<meta itemprop="price" content="99.90">`, "unverified_cash_price", 200},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			useTransport(t, func(r *http.Request) (*http.Response, error) { return response(r, tt.status, tt.body), nil })
			c := fetchPrice(Product{URL: magaluURL, Store: "Magazine Luiza"})
			if c.Diagnostic.Code != tt.code || c.Diagnostic.CheckedAt == "" || c.Diagnostic.HTTPStatus != tt.status {
				t.Fatalf("%+v", c)
			}
			if tt.code == "ok" {
				if c.Err != nil || c.Price == nil || *c.Price != 1029.99 {
					t.Fatalf("%+v", c)
				}
			} else if c.Err == nil || c.Price != nil {
				t.Fatalf("false price: %+v", c)
			}
		})
	}
}
func TestFetchNetworkRedirectAndUnsupportedStore(t *testing.T) {
	good := fixture(t, "magalu-pix.html")
	t.Run("network", func(t *testing.T) {
		useTransport(t, func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("test offline") })
		c := fetchPrice(Product{URL: magaluURL})
		if c.Diagnostic.Code != "network_error" || c.Price != nil {
			t.Fatalf("%+v", c)
		}
	})
	for _, raw := range []string{"https://www.magazineluiza.com.br/login", "https://www.magazineluiza.com.br/other/p/other/ed/ck4b/", "https://other.example/p/ed32c3cbdk/"} {
		t.Run(raw, func(t *testing.T) {
			useTransport(t, func(r *http.Request) (*http.Response, error) {
				u, _ := url.Parse(raw)
				copy := r.Clone(r.Context())
				copy.URL = u
				return response(copy, 200, good), nil
			})
			c := fetchPrice(Product{URL: magaluURL, Store: "Magazine Luiza"})
			if c.Err == nil || c.Price != nil {
				t.Fatalf("%+v", c)
			}
		})
	}
	for _, store := range []string{"Amazon", "Mercado Livre"} {
		t.Run(store, func(t *testing.T) {
			t.Setenv("MERCADOLIVRE_ACCESS_TOKEN", "fake-test-token")
			calls := 0
			useTransport(t, func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Header.Get("Authorization") != "" {
					t.Fatal("unexpected token")
				}
				return response(r, 200, good), nil
			})
			c := fetchPrice(Product{URL: magaluURL, Store: store})
			if c.Diagnostic.Code != "no_verified_method" || c.Price != nil || calls != 1 {
				t.Fatalf("%+v calls=%d", c, calls)
			}
		})
	}
}
func TestMonitorPersistsHistoryAndTelegram(t *testing.T) {
	good := fixture(t, "magalu-pix.html")
	dir := t.TempDir()
	old, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chdir(dir); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.Chdir(old) })
	os.MkdirAll("config", 0755)
	products := []Product{{ID: "philco", Name: "Philco", Store: "Magazine Luiza", URL: magaluURL, Target: 1100}, {ID: "dako", Store: "Mercado Livre", URL: "https://www.mercadolivre.com.br/p/MLB63741571", Target: 1100}}
	raw, _ := json.Marshal(products)
	os.WriteFile("config/products.json", raw, 0644)
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TELEGRAM_CHAT_ID", "test-chat")
	sent := 0
	useTransport(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "api.telegram.org" {
			sent++
			if r.Method != "POST" {
				t.Fatal("expected POST")
			}
			r.ParseForm()
			if r.Form.Get("chat_id") != "test-chat" || !strings.Contains(r.Form.Get("text"), "1029.99") {
				t.Fatalf("bad Telegram form %v", r.Form)
			}
			return response(r, 200, `{"ok":true}`), nil
		}
		if r.URL.Host == "www.mercadolivre.com.br" {
			return response(r, 403, "denied"), nil
		}
		return response(r, 200, good), nil
	})
	main()
	if sent != 1 {
		t.Fatalf("sent %d Telegram alerts", sent)
	}
	var history []Record
	raw, e = os.ReadFile("data/history.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &history); e != nil {
		t.Fatal(e)
	}
	if len(history) != 2 || history[0].Price == nil || *history[0].Price != 1029.99 || history[0].PaymentMethod != "pix" || history[1].Price != nil || history[1].Status != "unavailable" {
		t.Fatalf("%+v", history)
	}
	copied, e := os.ReadFile(filepath.Join("docs", "history.json"))
	if e != nil || string(copied) != string(raw) {
		t.Fatal("Pages history differs")
	}
	if _, e = os.Stat("docs/products.json"); e != nil {
		t.Fatal(e)
	}
	// Existing behavior: same-day samples replace rather than grow history.
	main()
	raw, _ = os.ReadFile("data/history.json")
	json.Unmarshal(raw, &history)
	if len(history) != 2 {
		t.Fatal("duplicate daily samples")
	}

	// A zero target keeps alerts disabled, including on a successful collection.
	before := sent
	products[0].Target = 0
	raw, _ = json.Marshal(products)
	os.WriteFile("config/products.json", raw, 0644)
	main()
	if sent != before {
		t.Fatal("zero target sent an alert")
	}
	// Preserve the existing suppression rule for a prior day already below target.
	products[0].Target = 1100
	raw, _ = json.Marshal(products)
	os.WriteFile("config/products.json", raw, 0644)
	previous := 1000.0
	history = append(history, Record{ID: "philco", At: "2000-01-01", Price: &previous, Status: "ok"})
	raw, _ = json.Marshal(history)
	os.WriteFile("data/history.json", raw, 0644)
	main()
	if sent != before {
		t.Fatal("prior below-target record did not suppress alert")
	}
}
