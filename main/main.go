package main

import (
	"github.com/daviddengcn/go-code-crawl"
	"github.com/daviddengcn/go-ljson-conf"
	"github.com/daviddengcn/gddo/doc"
	"log"
	"net/http"
	"net/url"
	"crypto/tls"
	"sync"
	"encoding/json"
	"time"
	"fmt"
)

var (
	serverAddr = "localhost:8080"
	proxyServer = ""
	restSeconds = 60
)

func init() {
	doc.SetGithubCredentials("94446b37edb575accd8b",
		"15f55815f0515a3f6ad057aaffa9ea83dceb220b")
	doc.SetUserAgent("Go-Code-Search-Agent")
}

func getJson(httpClient *http.Client, url string, v interface{}) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	dec := json.NewDecoder(resp.Body)
	
	return dec.Decode(v)
}

func main() {
	conf, _ := ljconf.Load("conf.json")
	
	serverAddr = conf.String("host", serverAddr)
	restSeconds = conf.Int("rest_seconds", restSeconds)
	proxyServer = conf.String("proxy", proxyServer)
	
	log.Printf("Server: %s", serverAddr)
	
	L := 10
	
	packageEntryURL := fmt.Sprintf("http://%s/crawlentries?kind=crawler&l=%d",
		serverAddr, L)
	personEntryURL := fmt.Sprintf("http://%s/crawlentries?kind=crawler-person&l=%d",
		serverAddr, L)
	
	packagePushURL := "http://" + serverAddr + "/pushpkg"
	personPushURL := "http://" + serverAddr + "/pushpsn"
	
	reportBadPackageURL := fmt.Sprintf("http://%s/reportbadpkg", serverAddr)
	//reportBadPersonURL := fmt.Sprintf("http://%s/reportbadPsn", serverAddr)
	
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
	
	httpClient := &http.Client {
		Transport: tp,
	}
	
	for {
		var wg sync.WaitGroup
		
		morePackages := false
		var pkgs []string
		err := getJson(httpClient, packageEntryURL, &pkgs)
		if err !=  nil {
			log.Printf("getJson(%s) failed: %v", packageEntryURL, err)
		} else {
			morePackages = len(pkgs) >= L
			
			groups := gcc.GroupPackages(pkgs)
			log.Printf("Packages: %v", groups)
			
			wg.Add(len(groups))
			
			for _, pkgs := range groups {
				go func(pkgs []string) {
					for _, pkg := range pkgs {
						p, err := gcc.CrawlPackage(httpClient, pkg)
						if err != nil {
							log.Printf("Crawling pkg %s failed: %v", pkg, err)
							
							if doc.IsNotFound(err) {
								// a wrong path
								err := gcc.ReportBadPackage(httpClient,
									reportBadPackageURL, pkg)
								log.Printf("Remove wrong package %s: %v", pkg, err)
							}
							continue
						}
						
						log.Printf("Crawled package %s success!", pkg)
						
						err = gcc.PushPackage(httpClient, packagePushURL, p)
						if err != nil {
							log.Printf("Push package %s failed: %v", pkg, err)
							continue
						}
						log.Printf("Push package %s success!", pkg)
					}
					
					wg.Done()
				}(pkgs)
			}
		}
		
		hasNewPackage := false
		var persons []string
		err = getJson(httpClient, personEntryURL, &persons)
		if err !=  nil {
			log.Printf("getJson(%s) failed: %v", personEntryURL, err)
		} else {
			groups := gcc.GroupPersons(persons)
			log.Printf("persons: %v", groups)
			
			wg.Add(len(groups))
			
			for _, ids := range groups {
				go func(ids []string) {
					for _, id := range ids {
						p, err := gcc.CrawlPerson(httpClient, id)
						if err != nil {
							log.Printf("Crawling person %s failed: %v", id, err)
							continue
						}
						
						log.Printf("Crawled person %s success!", id)
						reply, err := gcc.PushPerson(httpClient, personPushURL, p)
						if err != nil {
							log.Printf("Push person %s failed: %v", id, err)
							continue
						}
						
						log.Printf("Push person %s success: %+v", id, reply)
						if reply.NewPackage {
							hasNewPackage = true
						}
					}
					
					wg.Done()
				}(ids)
			}
		}
		
		wg.Wait()
		
		if !morePackages && !hasNewPackage {
			log.Printf("Nothing to do, have a rest...")
			time.Sleep(time.Duration(restSeconds) * time.Second)
		}
	}
}
