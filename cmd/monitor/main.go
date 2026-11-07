package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type Product struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Store  string  `json:"store"`
	URL    string  `json:"url"`
	Target float64 `json:"target_price"`
}
type Record struct {
	ID            string     `json:"id"`
	At            string     `json:"at"`
	Price         *float64   `json:"price"`
	Status        string     `json:"status"`
	Error         string     `json:"error,omitempty"`
	Source        string     `json:"source,omitempty"`
	Currency      string     `json:"currency,omitempty"`
	PaymentMethod string     `json:"payment_method,omitempty"`
	Seller        string     `json:"seller,omitempty"`
	Diagnostic    Diagnostic `json:"diagnostic,omitempty"`
}

var client = &http.Client{Timeout: 30 * time.Second}

func main() {
	raw, err := os.ReadFile("config/products.json")
	if err != nil {
		log.Fatal(err)
	}
	var products []Product
	if err = json.Unmarshal(raw, &products); err != nil {
		log.Fatal(err)
	}
	var history []Record
	if b, e := os.ReadFile("data/history.json"); e == nil {
		if e = json.Unmarshal(b, &history); e != nil {
			log.Fatal(e)
		}
	}
	today := time.Now().UTC().Format("2006-01-02")
	for _, p := range products {
		result := fetchPrice(p)
		price, source, err := result.Price, result.Source, result.Err
		rec := Record{ID: p.ID, At: today, Price: price, Status: "ok", Source: source, Diagnostic: result.Diagnostic}
		if err == nil {
			rec.Currency = "BRL"
			rec.PaymentMethod = "pix"
			rec.Seller = result.Seller
		}
		if err != nil {
			rec.Status = "unavailable"
			rec.Error = err.Error()
			log.Printf("%s [%s]: %v", p.ID, p.Store, err)
		} else {
			log.Printf("%s [%s]: R$ %.2f via %s", p.ID, p.Store, *price, source)
		}
		wasBelow := false
		for _, old := range history {
			if old.ID == p.ID && old.Price != nil && *old.Price <= p.Target && old.At < today {
				wasBelow = true
			}
		}
		replaced := false
		for i := range history {
			if history[i].ID == p.ID && history[i].At == today {
				history[i] = rec
				replaced = true
				break
			}
		}
		if !replaced {
			history = append(history, rec)
		}
		if err == nil && p.Target > 0 && *price <= p.Target && !wasBelow {
			notify(p, *price)
		}
	}
	writeJSON("data/history.json", history)
	writeJSON("docs/products.json", products)
	writeJSON("docs/history.json", history)
}
func writeJSON(path string, v any) {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(path, append(b, '\n'), 0644); e != nil {
		log.Fatal(e)
	}
}
func notify(p Product, price float64) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chat := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chat == "" {
		log.Printf("alerta para %s sem Telegram configurado", p.ID)
		return
	}
	msg := fmt.Sprintf("🔔 PromoProdutos: %s por R$ %.2f (meta R$ %.2f)\n%s", p.Name, price, p.Target, p.URL)
	form := url.Values{"chat_id": {chat}, "text": {msg}}
	res, e := client.PostForm("https://api.telegram.org/bot"+token+"/sendMessage", form)
	if e != nil {
		log.Printf("telegram: %v", e)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Printf("telegram HTTP %d", res.StatusCode)
	}
}
