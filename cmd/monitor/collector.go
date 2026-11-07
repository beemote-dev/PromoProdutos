package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Diagnostics contain no cookies, authorization headers or response bodies.
type Diagnostic struct {
	CheckedAt  string `json:"checked_at"`
	Method     string `json:"method"`
	Code       string `json:"code"`
	HTTPStatus int    `json:"http_status,omitempty"`
	FinalURL   string `json:"final_url,omitempty"`
	Bytes      int    `json:"bytes"`
	Title      string `json:"title,omitempty"`
}
type Collection struct {
	Price      *float64
	Source     string
	Seller     string
	Diagnostic Diagnostic
	Err        error
}

const maxPageBytes = 6 << 20

var scriptTagRE = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
var attrsRE = regexp.MustCompile(`(?is)([\w:-]+)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
var titleRE = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title>`)
var magaluIDRE = regexp.MustCompile(`/p/([a-zA-Z0-9]+)/`)

func fetchPrice(p Product) Collection {
	out := Collection{Diagnostic: Diagnostic{CheckedAt: time.Now().UTC().Format(time.RFC3339), Method: "go_net_http"}}
	fail := func(code, message string) Collection {
		out.Diagnostic.Code = code
		out.Err = fmt.Errorf("%s: %s", code, message)
		return out
	}
	requested, e := url.Parse(p.URL)
	if e != nil || requested.Scheme != "https" || requested.Host == "" || requested.User != nil {
		return fail("invalid_url", "URL HTTPS inválida")
	}
	req, e := http.NewRequest(http.MethodGet, p.URL, nil)
	if e != nil {
		return fail("invalid_url", e.Error())
	}
	// Same headers as the successful live Go probe. No cookies or challenge handling.
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")
	res, e := client.Do(req)
	if e != nil {
		return fail("network_error", e.Error())
	}
	defer res.Body.Close()
	out.Diagnostic.HTTPStatus = res.StatusCode
	out.Diagnostic.FinalURL = res.Request.URL.String()
	b, e := io.ReadAll(io.LimitReader(res.Body, maxPageBytes+1))
	out.Diagnostic.Bytes = len(b)
	if e != nil {
		return fail("read_error", e.Error())
	}
	if len(b) > maxPageBytes {
		return fail("body_too_large", "resposta excede 6 MiB; coleta recusada")
	}
	body := string(b)
	if m := titleRE.FindStringSubmatch(body); len(m) > 0 {
		title := strings.Join(strings.Fields(html.UnescapeString(m[1])), " ")
		if len(title) > 200 {
			title = title[:200]
		}
		out.Diagnostic.Title = title
	}
	if code := blockReason(res.StatusCode, res.Request.URL, body, out.Diagnostic.Title); code != "" {
		return fail(code, fmt.Sprintf("HTTP %d; página não utilizável para coleta", res.StatusCode))
	}
	if res.StatusCode != http.StatusOK {
		return fail("http_error", fmt.Sprintf("HTTP %d", res.StatusCode))
	}
	if res.Request.URL.Host != requested.Host || res.Request.URL.Scheme != "https" {
		return fail("unexpected_redirect", "redirecionamento para outro host ou protocolo")
	}
	if p.Store != "Magazine Luiza" || requested.Host != "www.magazineluiza.com.br" {
		return fail("no_verified_method", "nenhum método de preço à vista validado para esta loja; não usar metadados genéricos")
	}
	if magaluProductID(requested) != magaluProductID(res.Request.URL) {
		return fail("product_mismatch", "redirecionamento para outro produto")
	}
	price, seller, e := magaluPixPrice(body, requested)
	if e != nil {
		return fail("unverified_cash_price", e.Error())
	}
	out.Price = &price
	out.Seller = seller
	out.Source = "magalu_next_data_pix"
	out.Diagnostic.Code = "ok"
	return out
}

func blockReason(status int, u *url.URL, body, title string) string {
	lower := strings.ToLower(body)
	// Use challenge-specific markers; an incidental word in a product review is not a CAPTCHA.
	if strings.Contains(lower, `id="captchacharacters"`) || strings.Contains(lower, "/errors/validatecaptcha") || strings.Contains(strings.ToLower(title), "robot check") {
		return "captcha"
	}
	path := strings.ToLower(u.Path)
	if strings.Contains(path, "/login") || strings.Contains(path, "/signin") || strings.Contains(path, "/sign-in") || strings.Contains(path, "/registration") {
		return "login_required"
	}
	if status == 401 {
		return "authentication_required"
	}
	if status == 403 {
		return "access_denied"
	}
	if status == 429 {
		return "rate_limited"
	}
	if strings.Contains(lower, "não oferecemos suporte à sua sessão ou cliente") || strings.Contains(lower, "we don't support your session or client") {
		return "unsupported_client"
	}
	if status >= 500 {
		return "remote_error"
	}
	if strings.Contains(lower, "hubo un error accediendo a esta pagina") || strings.Contains(strings.ToLower(title), "access denied") {
		return "access_denied_page"
	}
	return ""
}

func attributes(raw string) map[string]string {
	attrs := map[string]string{}
	for _, m := range attrsRE.FindAllStringSubmatch(raw, -1) {
		v := m[2]
		if v == "" {
			v = m[3]
		}
		attrs[strings.ToLower(m[1])] = html.UnescapeString(v)
	}
	return attrs
}
func magaluProductID(u *url.URL) string {
	m := magaluIDRE.FindStringSubmatch(u.Path)
	if len(m) > 0 {
		return m[1]
	}
	return ""
}

type magaluItem struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
	Offers    []struct {
		Currency    string `json:"currency"`
		VariationID string `json:"variationId"`
		Seller      struct {
			ID        string `json:"id"`
			Available bool   `json:"available"`
			Active    bool   `json:"isActive"`
		} `json:"seller"`
		BestPrice struct {
			Method string      `json:"paymentMethodId"`
			Amount json.Number `json:"totalAmount"`
		} `json:"bestPrice"`
	} `json:"offers"`
}

// The only production parser: this exact path, currency, Pix method and seller
// were verified against both the real HTML and the rendered page on 2026-11-07.
func magaluPixPrice(body string, u *url.URL) (float64, string, error) {
	id, seller := magaluProductID(u), u.Query().Get("seller_id")
	if id == "" || seller == "" {
		return 0, "", fmt.Errorf("produto ou seller_id ausente na URL")
	}
	var payload string
	for _, m := range scriptTagRE.FindAllStringSubmatch(body, -1) {
		if attributes(m[1])["id"] == "__NEXT_DATA__" {
			if payload != "" {
				return 0, "", fmt.Errorf("estado principal ambíguo")
			}
			payload = m[2]
		}
	}
	if payload == "" {
		return 0, "", fmt.Errorf("__NEXT_DATA__ ausente")
	}
	var state struct {
		Props struct {
			PageProps struct {
				Data struct {
					Item magaluItem `json:"item"`
				} `json:"data"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	// Script JSON is raw text: HTML-unescaping can corrupt quoted descriptions.
	if e := json.Unmarshal([]byte(payload), &state); e != nil {
		return 0, "", fmt.Errorf("estado principal inválido: %w", e)
	}
	item := state.Props.PageProps.Data.Item
	if item.ID != id || !item.Available {
		return 0, "", fmt.Errorf("produto principal divergente ou indisponível")
	}
	var price float64
	matches := 0
	for _, o := range item.Offers {
		if o.Seller.ID != seller {
			continue
		}
		matches++
		if o.VariationID != id || !o.Seller.Active || !o.Seller.Available || o.Currency != "BRL" || o.BestPrice.Method != "pix" {
			return 0, "", fmt.Errorf("oferta principal sem confirmação de vendedor, variante, BRL e Pix")
		}
		v, e := strconv.ParseFloat(string(o.BestPrice.Amount), 64)
		if e != nil || math.IsInf(v, 0) || math.IsNaN(v) || v <= 0 || v >= 1000000 || math.Abs(v*100-math.Round(v*100)) > 1e-6 {
			return 0, "", fmt.Errorf("valor Pix inválido")
		}
		price = v
	}
	if matches != 1 {
		return 0, "", fmt.Errorf("esperava uma oferta do vendedor %s, recebi %d", seller, matches)
	}
	return price, seller, nil
}
