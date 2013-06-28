/* Fetch package list from api.godoc.org */
package main

import (
	"crypto/tls"
	"encoding/json"
	"github.com/daviddengcn/go-code-crawl"
	"github.com/daviddengcn/go-ljson-conf"
	"github.com/daviddengcn/ljson"
	"github.com/daviddengcn/go-rpc"
	"github.com/daviddengcn/go-villa"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

var (
	serverAddr  = "http://localhost:8080"
	proxyServer = ""
	
	doBlackPackages = false
	blackPkgFn      = villa.Path("black_pkgs.json")
	
	fastMode = false
	pushedFn = villa.Path("pushed_pkgs.json")
)

func init() {
	conf, _ := ljconf.Load("conf.json")
	serverAddr = conf.String("host", serverAddr)
	proxyServer = conf.String("proxy", proxyServer)

	doBlackPackages = conf.Bool("black_packages.enabled", doBlackPackages)
	if doBlackPackages {
		blackPkgFn = villa.Path(conf.String("black_packages.filename", blackPkgFn.S()))
	}
	
	fastMode = conf.Bool("godoc.fast_mode", fastMode)
	if fastMode {
		pushedFn = villa.Path(conf.String("godoc.pushed_pkg_fn", pushedFn.S()))
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

func loadPackages(fn villa.Path) ([]string, error) {
	f, err := fn.Open()
	if err != nil {
		return nil, villa.NestErrorf(err, "open file %s", fn)
	}
	defer f.Close()

	dec := ljson.NewDecoder(f)
	var list []string
	if err := dec.Decode(&list); err != nil {
		log.Printf("Decode %s failed: %v", fn, err)
		return nil, villa.NestErrorf(err, "decode list")
	}
	
	return list, nil
}

func savePackages(fn villa.Path, pkgs []string) error {
	f, err := fn.Create()
	if err != nil {
		return villa.NestErrorf(err, "open file %s", fn)
	}
	
	enc := json.NewEncoder(f)
	err = enc.Encode(pkgs)
	if err != nil {
		return villa.NestErrorf(err, "encoding packages")
	}
	return nil
}

func main() {
	const godocApiUrl = "http://api.godoc.org/packages"

	log.Printf("Server: %s", serverAddr)

	httpClient := genHttpClient(proxyServer)
	rpcClient := rpc.NewClient(httpClient, serverAddr)
	client := gcc.NewServiceClient(rpcClient)

	var blackPackages villa.StrSet
	if doBlackPackages {
		list, err := loadPackages(blackPkgFn)
		if err == nil {
			blackPackages.Put(list...)
			log.Printf("%d black packages loaded!", len(blackPackages))
		} else {
			log.Printf("Load packages from %s failed: %v", blackPkgFn, err)
		}
	}

	var pushedPackages villa.StrSet
	if fastMode {
		list, err := loadPackages(pushedFn)
		if err == nil {
			pushedPackages.Put(list...)
			log.Printf("%d pushed packages loaded!", len(pushedPackages))
		} else {
			log.Printf("Load packages from %s failed: %v", pushedFn, err)
		}
	}

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

	rand.Seed(time.Now().UnixNano())
	perms := rand.Perm(len(pkgs))

	pkgArr := make([]string, 0, len(pkgs))
	for i := range pkgs {
		pkg := pkgs[perms[i]]["path"]
		if blackPackages.In(pkg) || pushedPackages.In(pkg) {
			continue
		}
		pkgArr = append(pkgArr, pkg)
	}
	log.Printf("%d packages found!", len(pkgArr))

	newNum := 0
	appended := 0

	for len(pkgArr) > 0 {
		l := 200
		if len(pkgArr) < l {
			l = len(pkgArr)
		}

		nn := client.AppendPackages(nil, pkgArr[:l])
		err = client.LastError()
		if err != nil {
			log.Printf("AppendPackages failed: %v", err)
			break
		}
		
		pushedPackages.Put(pkgArr[:l]...)

		newNum += nn
		appended += l
		log.Printf("New packages: %d/%d", newNum, appended)

		pkgArr = pkgArr[l:]
	
		if fastMode {
			err := savePackages(pushedFn, pushedPackages.Elements())
			if err != nil {
				log.Printf("Save packages to %v failed: %v", pushedFn, err)
			} else {
				log.Printf("%d pushed packages saved to %s!", len(pushedPackages), pushedFn)
			}
		}
	}
}
