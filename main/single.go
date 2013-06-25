package main

import (
	"github.com/daviddengcn/go-code-crawl"
	"github.com/daviddengcn/go-ljson-conf"
	"github.com/daviddengcn/go-rpc"
	"github.com/daviddengcn/gddo/doc"
	"log"
	"net/http"
	"net/url"
	"crypto/tls"
	"os"
	"fmt"
)

var (
	serverAddr = "http://localhost:8080"
	proxyServer = ""
	restSeconds = 60
	entriesPerLoop = 10
)

func init() {
	doc.SetGithubCredentials("94446b37edb575accd8b",
		"15f55815f0515a3f6ad057aaffa9ea83dceb220b")
	doc.SetUserAgent("Go-Code-Search-Agent")
}

func showHelp() {
	fmt.Println("single <package|person> <path|id>")
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
	conf, _ := ljconf.Load("conf.json")
	
	serverAddr = conf.String("host", serverAddr)
	restSeconds = conf.Int("rest_seconds", restSeconds)
	proxyServer = conf.String("proxy", proxyServer)
	entriesPerLoop = conf.Int("entries_per_loop", entriesPerLoop)
	
	log.Printf("Server: %s", serverAddr)
	
	if len(os.Args) < 3 {
		showHelp()
		return
	}
	
	cmd := os.Args[1]
	if cmd != "package" && cmd != "person" {
		showHelp()
		return
	}
	
	httpClient := genHttpClient(proxyServer)
	rpcClient := rpc.NewClient(httpClient, serverAddr)
	client := gcc.NewServiceClient(rpcClient)
	
	switch cmd {
	case "package":
		pkg := os.Args[2]
		p, err := gcc.CrawlPackage(httpClient, pkg)
		if err != nil {
			log.Printf("Crawling pkg %s failed: %v", pkg, err)
			
			if doc.IsNotFound(err) {
				// a wrong path
				client.ReportBadPackage(nil, pkg)
				log.Printf("Remove wrong package %s: %v", pkg, client.LastError)
			}
			break
		}
		
		log.Printf("Crawled package %s success!", pkg)
		log.Printf("Imports: %d, Doc: %d, ReadmeFn: %s, ReadmeData: %d",
			len(p.Imports), len(p.Doc), p.ReadmeFn, len(p.ReadmeData))
		
		client.PushPackage(nil, p)
		err = client.LastError()
		if err != nil {
			log.Printf("Push package %s failed: %v", pkg, err)
			break
		}
		log.Printf("Push package %s success!", pkg)
		
	case "person":
		id := os.Args[2]
		p, err := gcc.CrawlPerson(httpClient, id)
		if err != nil {
			log.Printf("Crawling person %s failed: %v", id, err)
			break
		}
		
		log.Printf("Crawled person %s success!", id)
		newPackage := client.PushPerson(nil, p)
		err = client.LastError()
		if err != nil {
			log.Printf("Push person %s failed: %v", id, err)
			break
		}
		
		log.Printf("Push person %s success: %v", id, newPackage)
	}
}
