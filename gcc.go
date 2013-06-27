/*
	Package gcc is the crawling package for go-code search engine (GCSE)
*/
package gcc

import (
	"fmt"
	"github.com/daviddengcn/gddo/doc"
	godoc "go/doc"
	"github.com/daviddengcn/go-villa"
	"net/http"
	"net/url"
	"strings"

	"github.com/daviddengcn/go-rpc"
	
	"unicode/utf8"
)

type Package struct {
	Name       string
	ImportPath string
	Synopsis   string
	Doc        string
	ProjectURL string

	StarCount  int
	ReadmeFn   string
	ReadmeData string

	Imports    []string
	References []string
}

func CrawlPackage(httpClient *http.Client, pkg string) (p *Package, err error) {
	pdoc, err := doc.Get(httpClient, pkg, "")
	if err != nil {
		return nil, villa.NestErrorf(err, "CrawlPackage(%s)", pkg)
	}

	readmeFn, readmeData := "", ""
	for fn, data := range pdoc.ReadmeFiles {
		readmeFn, readmeData = fn, string(data)
		if utf8.ValidString(readmeData) {
			break
		} else {
			readmeFn, readmeData = "", ""
		}
	}
	
	if pdoc.Doc == "" && pdoc.Synopsis == "" {
		pdoc.Synopsis = godoc.Synopsis(readmeData)
	}
	
	return &Package{
		Name:       pdoc.Name,
		ImportPath: pdoc.ImportPath,
		Synopsis:   pdoc.Synopsis,
		Doc:        pdoc.Doc,
		ProjectURL: pdoc.ProjectURL,
		StarCount:  pdoc.StarCount,

		ReadmeFn:   readmeFn,
		ReadmeData: readmeData,

		Imports:    pdoc.Imports,
		References: pdoc.References,
	}, nil
}

func IdOfPerson(site, username string) string {
	return fmt.Sprintf("%s:%s", site, username)
}

func ParsePersonId(id string) (site, username string) {
	parts := strings.Split(id, ":")
	return parts[0], parts[1]
}

func GroupPackages(pkgs []string) (groups map[string][]string) {
	groups = make(map[string][]string)

	for _, pkg := range pkgs {
		host := ""
		u, err := url.Parse("http://" + pkg)
		if err == nil {
			host = u.Host
		}

		groups[host] = append(groups[host], pkg)
	}

	return
}

func GroupPersons(ids []string) (groups map[string][]string) {
	groups = make(map[string][]string)

	for _, id := range ids {
		host, _ := ParsePersonId(id)

		groups[host] = append(groups[host], id)
	}

	return
}

type Person struct {
	Id       string
	Packages []string
}

func CrawlPerson(httpClient *http.Client, id string) (*Person, error) {
	site, username := ParsePersonId(id)
	switch site {
	case "github.com":
		p, err := doc.GetGithubPerson(httpClient, map[string]string{"owner": username})
		if err != nil {
			return nil, villa.NestErrorf(err, "CrawlPerson(%s)", id)
		} else {
			return &Person{
				Id:       id,
				Packages: p.Projects,
			}, nil
		}
	case "bitbucket.org":
		p, err := doc.GetBitbucketPerson(httpClient, map[string]string{"owner": username})
		if err != nil {
			return nil, villa.NestErrorf(err, "CrawlPerson(%s)", id)
		} else {
			return &Person{
				Id:       id,
				Packages: p.Projects,
			}, nil
		}
	}

	return nil, nil
}

func IsBadPackage(err error) bool {
	return doc.IsNotFound(villa.DeepestNested(err))
}

/*
	For client side, the first parameter (*http.Request) is ignored, simply set
	it to nil.
*/
type GoSearchService interface {
	FetchPackageList(r *http.Request, l int) (pkgs []string)
	PushPackage(r *http.Request, p *Package)
	ReportBadPackage(r *http.Request, pkg string)
	TouchPackage(r *http.Request, pkg string) (earlySchedule bool)
	AppendPackages(r *http.Request, pkgs []string) (newNum int)
	
	FetchPersonList(r *http.Request, l int) (ids []string)
	PushPerson(r *http.Request, p *Person) (NewPackage bool)
	
	LastError() error
}

func Register(server GoSearchService) {
	rpc.Register(server)
}

type client struct {
	lastError error
	rpcClient *rpc.Client
}

func (c *client) FetchPackageList(r *http.Request, l int) (pkgs []string) {
	c.lastError = c.rpcClient.Call(1, "FetchPackageList", l, &pkgs)
	return
}

func (c *client) FetchPersonList(r *http.Request, l int) (ids []string) {
	c.lastError = c.rpcClient.Call(1, "FetchPersonList", l, &ids)
	return
}

func (c *client) PushPackage(r *http.Request, p *Package) {
	c.lastError = c.rpcClient.Call(1, "PushPackage", p)
}

func (c *client) ReportBadPackage(r *http.Request, pkg string) {
	c.lastError = c.rpcClient.Call(1, "ReportBadPackage", pkg)
}

func (c *client) PushPerson(r *http.Request, p *Person) (NewPackage bool) {
	c.lastError = c.rpcClient.Call(1, "PushPerson", p, &NewPackage)
	return
}

func (c *client) TouchPackage(r *http.Request, pkg string) (earlySchedule bool) {
	c.lastError = c.rpcClient.Call(1, "TouchPackage", pkg, &earlySchedule)
	return
}

func (c *client) AppendPackages(r *http.Request, pkgs []string) (newNum int) {
	c.lastError = c.rpcClient.Call(1, "AppendPackages", pkgs, &newNum)
	return
}

func (c *client) LastError() error {
	return c.lastError
}

func NewServiceClient(rpcClient *rpc.Client) GoSearchService {
	return &client{rpcClient: rpcClient}
}
