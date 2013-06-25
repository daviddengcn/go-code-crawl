/* Fetch package list from api.godoc.org */
package main

import (
	"github.com/daviddengcn/go-code-crawl"
	"github.com/daviddengcn/go-ljson-conf"
	"github.com/daviddengcn/go-rpc"
	"log"
	"net/http"
	"net/url"
	"crypto/tls"
	"encoding/json"
)

var (
	serverAddr = "http://localhost:8080"
	proxyServer = ""
)

func init() {
	conf, _ := ljconf.Load("conf.json")
	serverAddr = conf.String("host", serverAddr)
	proxyServer = conf.String("proxy", proxyServer)
	
}

func genHttpClient(proxy string) *http.Client {
	tp := &http.Transport {
		TLSClientConfig: &tls.Config {
			InsecureSkipVerify: true,
		},
	}
	if proxyServer != "" {
		proxyURL, err := url.Parse(proxyServer)
		if err != nil {
			log.Printf("Parsing proxy host failed: %v", err)
		} else {
			log.Printf("Using proxy: %v", proxyURL)
			tp.Proxy = http.ProxyURL(proxyURL)
		}
	}
	
	return &http.Client {
		Transport: tp,
	}
}


func main() {
	const godocApiUrl = "http://api.godoc.org/packages"
	
	log.Printf("Server: %s", serverAddr)
	
	httpClient := genHttpClient(proxyServer)
	rpcClient := rpc.NewClient(httpClient, serverAddr)
	client := gcc.NewServiceClient(rpcClient)
	
	log.Printf("Crawling %s ...", godocApiUrl)
	resp, err := httpClient.Get(godocApiUrl)
	if err != nil {
		log.Printf("Get %s failed: %v", godocApiUrl, err)
		return
	}
	if resp.StatusCode != 200 {
		log.Printf("StatusCode: %d", resp.StatusCode)
		return
	}
	defer resp.Body.Close()
	
	var results map[string][]map[string]string
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&results)
	if err != nil {
		log.Printf("Parse results failed: %v", err)
		return
	}
	
	pkgs := results["results"]
	log.Printf("%d packages found!", len(pkgs))
	
	pkgArr := make([]string, len(pkgs))
	for i := range pkgs {
		pkgArr[i] = pkgs[i]["path"]
	}
	
	newNum := 0
	
	for i := 0; i < len(pkgArr); i += 200 {
		l := 200
		if len(pkgArr) < l {
			l = len(pkgArr)
		}
		
		nn := client.AppendPackages(nil, pkgArr[:l])
		err = client.LastError()
		if err != nil {
			log.Printf("AppendPackages failed: %v", err)
			return
		}
		
		pkgArr = pkgArr[l:]
		newNum += nn
		log.Printf("New packages: %d", newNum)
	}
}
