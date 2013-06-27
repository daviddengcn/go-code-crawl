package main

import (
	"crypto/tls"
	"github.com/daviddengcn/gddo/doc"
	"github.com/daviddengcn/go-code-crawl"
	"github.com/daviddengcn/go-ljson-conf"
	"github.com/daviddengcn/go-rpc"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var (
	serverAddr     = "http://localhost:8080"
	proxyServer    = ""
	restSeconds    = 60
	entriesPerLoop = 10
)

func init() {
	doc.SetGithubCredentials("94446b37edb575accd8b",
		"15f55815f0515a3f6ad057aaffa9ea83dceb220b")
	doc.SetUserAgent("Go-Code-Search-Agent")
}

func main() {
	conf, _ := ljconf.Load("conf.json")

	serverAddr = conf.String("host", serverAddr)
	restSeconds = conf.Int("rest_seconds", restSeconds)
	proxyServer = conf.String("proxy", proxyServer)
	entriesPerLoop = conf.Int("entries_per_loop", entriesPerLoop)

	log.Printf("Server: %s", serverAddr)

	httpClient := genHttpClient(proxyServer)
	rpcClient := rpc.NewClient(httpClient, serverAddr)
	client := gcc.NewServiceClient(rpcClient)

	for {
		var wg sync.WaitGroup

		morePackages := false
		pkgs := client.FetchPackageList(nil, entriesPerLoop)
		err := client.LastError()
		if err != nil {
			log.Printf("FetchPackageList failed: %v", err)
		} else {
			morePackages = len(pkgs) >= entriesPerLoop

			groups := gcc.GroupPackages(pkgs)
			log.Printf("Packages: %v, %d groups", groups, len(groups))

			wg.Add(len(groups))

			for _, pkgs := range groups {
				go func(pkgs []string) {
					for _, pkg := range pkgs {
						p, err := gcc.CrawlPackage(httpClient, pkg)
						if err != nil {
							log.Printf("Crawling pkg %s failed: %v", pkg, err)

							if gcc.IsBadPackage(err) {
								// a wrong path
								client.ReportBadPackage(nil, pkg)
								log.Printf("Remove wrong package %s: %v", pkg, client.LastError())
							}
							continue
						}

						log.Printf("Crawled package %s success!", pkg)

						client.PushPackage(nil, p)
						err = client.LastError()
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
		morePersons := false
		persons := client.FetchPersonList(nil, entriesPerLoop)
		err = client.LastError()
		if err != nil {
			log.Printf("FetchPersonList failed: %v", err)
		} else {
			morePersons = len(persons) >= entriesPerLoop

			groups := gcc.GroupPersons(persons)
			log.Printf("persons: %v, %d groups", groups, len(groups))

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
						newPackage := client.PushPerson(nil, p)
						err = client.LastError()
						if err != nil {
							log.Printf("Push person %s failed: %v", id, err)
							continue
						}

						log.Printf("Push person %s success: %v", id, newPackage)
						if newPackage {
							hasNewPackage = true
						}
					}

					wg.Done()
				}(ids)
			}
		}

		wg.Wait()

		if !morePackages && !morePersons && !hasNewPackage {
			log.Printf("Nothing to do, have a rest...")
			time.Sleep(time.Duration(restSeconds) * time.Second)
		}
	}
}

func genHttpClient(proxy string) *http.Client {
	tp := &http.Transport{
		TLSClientConfig: &tls.Config{
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

	return &http.Client{
		Transport: tp,
	}
}
