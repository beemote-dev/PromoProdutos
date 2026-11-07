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
type Record struct { ID string `json:"id"`; At string `json:"at"`; Price *float64 `json:"price"`; Status string `json:"status"`; Error string `json:"error,omitempty"`; Source string `json:"source,omitempty"` }
var scriptRE=regexp.MustCompile(`(?is)<script\b[^>]*type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)
var metaRE=regexp.MustCompile(`(?is)<meta\b[^>]*>`)
var attrsRE=regexp.MustCompile(`(?is)([\w:-]+)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
var client=&http.Client{Timeout:30*time.Second}
func main(){
 raw,err:=os.ReadFile("config/products.json");if err!=nil{log.Fatal(err)}
 var products []Product;if err=json.Unmarshal(raw,&products);err!=nil{log.Fatal(err)}
 var history []Record
 if b,e:=os.ReadFile("data/history.json");e==nil{if e=json.Unmarshal(b,&history);e!=nil{log.Fatal(e)}}
 today:=time.Now().UTC().Format("2006-01-02")
 for _,p:=range products{
  price,source,err:=fetchPrice(p)
  rec:=Record{ID:p.ID,At:today,Price:price,Status:"ok",Source:source}
  if err!=nil{rec.Status="unavailable";rec.Error=err.Error();log.Printf("%s [%s]: %v",p.ID,p.Store,err)}else{log.Printf("%s [%s]: R$ %.2f via %s",p.ID,p.Store,*price,source)}
  wasBelow:=false
  for _,old:=range history{if old.ID==p.ID && old.Price!=nil && *old.Price<=p.Target && old.At<today{wasBelow=true}}
  replaced:=false
  for i:=range history{if history[i].ID==p.ID && history[i].At==today{history[i]=rec;replaced=true;break}}
  if !replaced{history=append(history,rec)}
  if err==nil && p.Target>0 && *price<=p.Target && !wasBelow{notify(p,*price)}
 }
 writeJSON("data/history.json",history);writeJSON("docs/products.json",products);writeJSON("docs/history.json",history)
}
func fetchPrice(p Product)(*float64,string,error){
 if p.Store=="Mercado Livre" && os.Getenv("MERCADOLIVRE_ACCESS_TOKEN")!=""{
  itemID:="MLB4480279587"
  if v,e:=fetchML(itemID);e==nil{return &v,"mercadolivre_api",nil}else{log.Printf("Mercado Livre API: %v; tentando página",e)}
 }
 req,e:=http.NewRequest("GET",p.URL,nil);if e!=nil{return nil,"",e}
 req.Header.Set("User-Agent","Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
 req.Header.Set("Accept","text/html,application/xhtml+xml")
 req.Header.Set("Accept-Language","pt-BR,pt;q=0.9")
 res,e:=client.Do(req);if e!=nil{return nil,"",e};defer res.Body.Close()
 if res.StatusCode!=200{return nil,"",fmt.Errorf("página retornou HTTP %d (possível bloqueio ou indisponibilidade)",res.StatusCode)}
 b,e:=io.ReadAll(io.LimitReader(res.Body,6<<20));if e!=nil{return nil,"",e}
 body:=string(b)
 for _,m:=range scriptRE.FindAllStringSubmatch(body,-1){
  var obj any
  if json.Unmarshal([]byte(html.UnescapeString(m[1])),&obj)==nil{
   if v,ok:=findOfferPrice(obj);ok{return &v,"json_ld",nil}
  }
 }
 // Somente metadados explicitamente identificados como preço de produto.
 for _,tag:=range metaRE.FindAllString(body,-1){
  attrs:=map[string]string{}
  for _,a:=range attrsRE.FindAllStringSubmatch(tag,-1){value:=a[2];if value==""{value=a[3]};attrs[strings.ToLower(a[1])]=html.UnescapeString(value)}
  key:=strings.ToLower(attrs["property"]);if key==""{key=strings.ToLower(attrs["itemprop"])}
  if key==""{key=strings.ToLower(attrs["name"])}
  if key=="product:price:amount" || key=="product:sale_price:amount" || key=="price"{
   value:=attrs["content"]
   if v,ok:=parsePrice(value);ok{return &v,"meta_price",nil}
  }
 }
 return nil,"",fmt.Errorf("HTTP 200, mas não encontrei preço confiável em JSON-LD ou metadados (%d bytes); página pode exigir JavaScript ou bloquear robôs",len(b))
}
func fetchML(id string)(float64,error){
 token:=os.Getenv("MERCADOLIVRE_ACCESS_TOKEN")
 req,e:=http.NewRequest("GET","https://api.mercadolibre.com/items/"+url.PathEscape(id)+"/prices",nil);if e!=nil{return 0,e}
 req.Header.Set("Authorization","Bearer "+token)
 res,e:=client.Do(req);if e!=nil{return 0,e};defer res.Body.Close()
 if res.StatusCode!=200{return 0,fmt.Errorf("HTTP %d",res.StatusCode)}
 var data struct{Prices []struct{Type string `json:"type"`;Amount float64 `json:"amount"`;Currency string `json:"currency_id"`;Conditions struct{Context []string `json:"context_restrictions"`} `json:"conditions"`} `json:"prices"`}
 if e=json.NewDecoder(io.LimitReader(res.Body,1<<20)).Decode(&data);e!=nil{return 0,e}
 // Somente preços gerais sem restrições de contexto.
 var standard,promo float64
 for _,v:=range data.Prices{if v.Currency!="BRL"||v.Amount<=0||len(v.Conditions.Context)>0{continue};if v.Type=="standard"{standard=v.Amount};if v.Type=="promotion"&&(promo==0||v.Amount<promo){promo=v.Amount}}
 if promo>0{return promo,nil};if standard>0{return standard,nil}
 return 0,fmt.Errorf("API não retornou preço BRL público válido")
}
func findOfferPrice(v any)(float64,bool){
 switch x:=v.(type){
 case []any:for _,a:=range x{if p,ok:=findOfferPrice(a);ok{return p,true}}
 case map[string]any:
  t:=fmt.Sprint(x["@type"])
  if strings.Contains(strings.ToLower(t),"product"){if offers,ok:=x["offers"];ok{if p,ok:=extractOffer(offers);ok{return p,true}}}
  if graph,ok:=x["@graph"];ok{return findOfferPrice(graph)}
 }
 return 0,false
}
func extractOffer(v any)(float64,bool){
 switch x:=v.(type){
 case []any:for _,a:=range x{if p,ok:=extractOffer(a);ok{return p,true}}
 case map[string]any:
  currency,_:=x["priceCurrency"].(string);if currency!=""&&currency!="BRL"{return 0,false}
  if raw,ok:=x["price"];ok{if p,ok:=parsePrice(raw);ok{return p,true}}
  if spec,ok:=x["priceSpecification"];ok{return extractOffer(spec)}
 }
 return 0,false
}
func parsePrice(v any)(float64,bool){
 s:=strings.TrimSpace(fmt.Sprint(v));s=strings.ReplaceAll(s,"R$","");s=strings.ReplaceAll(s,"\u00a0","");s=strings.TrimSpace(s)
 if strings.Contains(s,","){s=strings.ReplaceAll(s,".","");s=strings.ReplaceAll(s,",",".")}
 p,e:=strconv.ParseFloat(s,64);return p,e==nil&&p>0&&p<1000000
}
func writeJSON(path string,v any){
 b,e:=json.MarshalIndent(v,"","  ");if e!=nil{log.Fatal(e)}
 if e=os.MkdirAll(filepath.Dir(path),0755);e!=nil{log.Fatal(e)}
 if e=os.WriteFile(path,append(b,'\n'),0644);e!=nil{log.Fatal(e)}
}
func notify(p Product,price float64){
 token:=os.Getenv("TELEGRAM_BOT_TOKEN");chat:=os.Getenv("TELEGRAM_CHAT_ID")
 if token==""||chat==""{log.Printf("alerta para %s sem Telegram configurado",p.ID);return}
 msg:=fmt.Sprintf("🔔 PromoProdutos: %s por R$ %.2f (meta R$ %.2f)\n%s",p.Name,price,p.Target,p.URL)
 form:=url.Values{"chat_id":{chat},"text":{msg}}
 res,e:=client.PostForm("https://api.telegram.org/bot"+token+"/sendMessage",form)
 if e!=nil{log.Printf("telegram: %v",e);return};defer res.Body.Close()
 if res.StatusCode!=200{log.Printf("telegram HTTP %d",res.StatusCode)}
}
