// Live diagnostics only. Never sends alerts or changes price history.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type target struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func main() {
	output := flag.String("output", "work/probes", "diagnostic directory (raw public responses included)")
	flag.Parse()
	raw, e := os.ReadFile("config/products.json")
	must(e)
	var targets []target
	must(json.Unmarshal(raw, &targets))
	must(os.MkdirAll(*output, 0755))
	targets = append(targets, target{"ml-product", "https://api.mercadolibre.com/products/MLB63741571"}, target{"ml-items", "https://api.mercadolibre.com/products/MLB63741571/items"})
	c := http.Client{Timeout: 40 * time.Second}
	for _, t := range targets {
		record := map[string]any{"id": t.ID, "checked_at": time.Now().UTC().Format(time.RFC3339), "requested_url": t.URL, "method": "go_net_http"}
		req, e := http.NewRequest(http.MethodGet, t.URL, nil)
		must(e)
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")
		res, e := c.Do(req)
		if e != nil {
			record["error"] = e.Error()
		} else {
			b, readErr := io.ReadAll(io.LimitReader(res.Body, 8<<20))
			res.Body.Close()
			record["status"] = res.StatusCode
			record["final_url"] = res.Request.URL.String()
			record["bytes"] = len(b)
			record["sha256"] = fmt.Sprintf("%x", sha256.Sum256(b))
			headers := map[string]string{}
			for _, k := range []string{"Content-Type", "Server", "X-Cache", "Rps", "Retry-After"} {
				if v := res.Header.Get(k); v != "" {
					headers[k] = v
				}
			}
			record["headers"] = headers
			if readErr != nil {
				record["error"] = readErr.Error()
			}
			must(os.WriteFile(filepath.Join(*output, t.ID+"-go.body"), b, 0644))
		}
		b, e := json.MarshalIndent(record, "", "  ")
		must(e)
		must(os.WriteFile(filepath.Join(*output, t.ID+"-go.json"), append(b, '\n'), 0644))
		fmt.Println(string(b))
	}
}
func must(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
