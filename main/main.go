package main

import (
	"github.com/daviddengcn/go-code-crawl"
	"github.com/daviddengcn/go-ljson-conf"
	"log"
	"net/http"
	"crypto/tls"
	"sync"
	"encoding/json"
	"time"
)

var (
	serverAddr = "localhost:8080"
	restSeconds = 60
)

func getJson(url string, v interface{}) error {
	resp, err := http.Get(url)
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
	
	log.Printf("Server: %s", serverAddr)
	
	packageEntryURL := "http://" + serverAddr + "/crawlentries?kind=crawler"
	personEntryURL := "http://" + serverAddr + "/crawlentries?kind=crawler-person"
	
	packagePushURL := "http://" + serverAddr + "/pushpkg"
	personPushURL := "http://" + serverAddr + "/pushpsn"
	
	httpClient := &http.Client {
		Transport: &http.Transport {
			TLSClientConfig: &tls.Config {
				InsecureSkipVerify: true,
			},
		},
	}
	
	for {
		var wg sync.WaitGroup
		
		var pkgs []string
		err := getJson(packageEntryURL, &pkgs)
		if err !=  nil {
			log.Printf("getJson(%s) failed: %v", packageEntryURL, err)
		} else {
			groups := gcc.GroupPackages(pkgs)
			log.Printf("Packages: %v", groups)
			
			wg.Add(len(groups))
			
			for _, pkgs := range groups {
				go func(pkgs []string) {
					for _, pkg := range pkgs {
						p, err := gcc.CrawlPackage(httpClient, pkg)
						if err != nil {
							log.Printf("Crawling pkg %s failed: %v", pkg, err)
							continue
						}
						
						log.Printf("Crawled package %s success!", pkg)
						
						err = gcc.PushPackage(httpClient, packagePushURL, p)
						if err != nil {
							log.Printf("Push package %s failed: %v", pkg, err)
						}
					}
					
					wg.Done()
				}(pkgs)
			}
		}
		
		hasNewPackage := false
		var persons []string
		err = getJson(personEntryURL, &persons)
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
		
		if !hasNewPackage {
			log.Printf("Nothing to do, have a rest...")
			time.Sleep(time.Duration(restSeconds) * time.Second)
		}
	}
}
