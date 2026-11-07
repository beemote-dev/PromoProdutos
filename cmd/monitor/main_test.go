package main

import "testing"

func TestAmazonMainPrice(t *testing.T) {
 html := `<div id="recommendations"><span class="a-offscreen">R$ 99,90</span></div>
 <div id="corePriceDisplay_desktop_feature_div"><span class="a-price"><span class="a-offscreen">R$ 1.299,90</span></span></div>`
 got, ok := findAmazonPrice(html)
 if !ok || got != 1299.90 { t.Fatalf("got %v, ok %v", got, ok) }
}

func TestAmazonNoOtherOffers(t *testing.T) {
 html := `<div id="recommendations"><span class="a-offscreen">R$ 99,90</span></div>`
 if v, ok := findAmazonPrice(html); ok { t.Fatalf("unexpected price %v", v) }
}

func TestParsePrices(t *testing.T) {
 cases := map[string]float64{"1.099,99": 1099.99, "1099.99": 1099.99, "R$ 1.234,50": 1234.50}
 for input, expected := range cases { got,ok:=parsePrice(input);if !ok||got!=expected {t.Errorf("%q: %v (%v), expected %v",input,got,ok,expected)} }
}
