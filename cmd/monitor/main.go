package main

import (
 "encoding/json"
 "fmt"
 "html"
 "io"
 "log"
 "net/http"
 "net/url"
 "os"
 "path/filepath"
 "regexp"
 "strconv"
 "strings"
 "time"
)

type Product struct { ID string `json:"id"`; Name string `json:"name"`; Store string `json:"store"`; URL string `json:"url"`; Target float64 `json:"target_price"` }
type Record struct { ID string `json:"id"`; At string `json:"at"`; Price *float64 `json:"price"`; Status string `json:"status"` }
var scriptRE=regexp.MustCompile(`(?is)<script[^>]*type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)
var client=&http.Client{Timeout:25*time.Second}
func main(){
 raw,err:=os.ReadFile("config/products.json"); if err!=nil {log.Fatal(err)}
 var products []Product; if err=json.Unmarshal(raw,&products);err!=nil {log.Fatal(err)}
 var history []Record
 if b,e:=os.ReadFile("data/history.json");e==nil {if e=json.Unmarshal(b,&history);e!=nil {log.Fatal(e)}}
 today:=time.Now().UTC().Format("2006-01-02")
 for _,p:=range products {
  price,err:=fetchPrice(p.URL)
  rec:=Record{ID:p.ID,At:today,Status:"ok",Price:price}
  if err!=nil {rec.Status="unavailable";log.Printf("%s: %v",p.ID,err)}
  replaced:=false
  for i:=range history {if history[i].ID==p.ID && history[i].At==today {history[i]=rec;replaced=true;break}}
  if !replaced {history=append(history,rec)}
  if err==nil && p.Target>0 && *price<=p.Target && !previousAlert(history,p.ID,today,p.Target) {notify(p,*price)}
 }
 writeJSON("data/history.json",history)
 writeJSON("docs/products.json",products)
 writeJSON("docs/history.json",history)
}
func previousAlert(history []Record,id,today string,target float64) bool {
 for _,r:=range history {if r.ID==id && r.At<today && r.Price!=nil && *r.Price<=target {return true}}
 return false
}
func fetchPrice(link string)(*float64,error){
 req,e:=http.NewRequest("GET",link,nil);if e!=nil{return nil,e}
 req.Header.Set("User-Agent","Mozilla/5.0 (compatible; PromoProdutos/1.0)")
 req.Header.Set("Accept","text/html,application/xhtml+xml")
 res,e:=client.Do(req);if e!=nil{return nil,e};defer res.Body.Close()
 if res.StatusCode!=200{return nil,fmt.Errorf("HTTP %d",res.StatusCode)}
 b,e:=io.ReadAll(io.LimitReader(res.Body,4<<20));if e!=nil{return nil,e}
 for _,m:=range scriptRE.FindAllStringSubmatch(string(b),-1){
  var obj any
  if json.Unmarshal([]byte(html.UnescapeString(m[1])),&obj)==nil {
   if v,ok:=findOfferPrice(obj);ok{return &v,nil}
  }
 }
 return nil,fmt.Errorf("nenhum preço estruturado confiável encontrado")
}
func findOfferPrice(v any)(float64,bool){
 switch x:=v.(type){
 case []any: for _,a:=range x {if p,ok:=findOfferPrice(a);ok{return p,true}}
 case map[string]any:
  if offers,ok:=x["offers"];ok {if p,ok:=extractOffer(offers);ok{return p,true}}
  if graph,ok:=x["@graph"];ok{return findOfferPrice(graph)}
 }
 return 0,false
}
func extractOffer(v any)(float64,bool){
 switch x:=v.(type){
 case []any:for _,a:=range x {if p,ok:=extractOffer(a);ok{return p,true}}
 case map[string]any:
  currency,_:=x["priceCurrency"].(string)
  if currency!="" && currency!="BRL" {return 0,false}
  if raw,ok:=x["price"];ok {if p,ok:=parsePrice(raw);ok{return p,true}}
  if spec,ok:=x["priceSpecification"];ok{return extractOffer(spec)}
 }
 return 0,false
}
func parsePrice(v any)(float64,bool){
 s:=strings.TrimSpace(fmt.Sprint(v));if strings.Contains(s,",") {s=strings.ReplaceAll(s,".","");s=strings.ReplaceAll(s,",",".")}
 p,e:=strconv.ParseFloat(s,64);return p,e==nil&&p>0
}
func writeJSON(path string,v any){
 b,e:=json.MarshalIndent(v,"","  ");if e!=nil {log.Fatal(e)}
 if e=os.MkdirAll(filepath.Dir(path),0755);e!=nil{log.Fatal(e)}
 if e=os.WriteFile(path,append(b,'\n'),0644);e!=nil{log.Fatal(e)}
}
func notify(p Product,price float64){
 token:=os.Getenv("TELEGRAM_BOT_TOKEN");chat:=os.Getenv("TELEGRAM_CHAT_ID")
 if token==""||chat=="" {log.Printf("alerta para %s sem Telegram configurado",p.ID);return}
 msg:=fmt.Sprintf("🔔 PromoProdutos: %s por R$ %.2f (meta R$ %.2f)\n%s",p.Name,price,p.Target,p.URL)
 form:=url.Values{"chat_id":{chat},"text":{msg}}
 res,e:=client.PostForm("https://api.telegram.org/bot"+token+"/sendMessage",form)
 if e!=nil {log.Printf("telegram: %v",e);return};defer res.Body.Close()
 if res.StatusCode!=200 {log.Printf("telegram HTTP %d",res.StatusCode)}
}
