/*
	Package gcc is the crawling package for go-code search engine (GCSE)
 */
package gcc

import (
	"fmt"
	"strings"
	"net/http"
	"net/url"
	"github.com/daviddengcn/gddo/doc"
	"encoding/json"
)

type Package struct {
	Name string
	ImportPath string
	Synopsis string
	Doc string
	ProjectURL string
	
	StarCount int
	ReadmeFn string
	ReadmeData string
	
	Imports []string
	References []string
}

func CrawlPackage(httpClient *http.Client, pkg string) (p *Package, err error) {
	pdoc, err := doc.Get(httpClient, pkg, "")
	if err != nil {
		return nil, err
	}
	
	readmeFn, readmeData := "", ""
	for fn, data := range pdoc.ReadmeFiles {
		readmeFn, readmeData = fn, string(data)
		break
	}
	
	return &Package {
		Name: pdoc.Name,
		ImportPath: pdoc.ImportPath,
		Synopsis: pdoc.Synopsis,
		Doc: pdoc.Doc,
		ProjectURL: pdoc.ProjectURL,
		StarCount: pdoc.StarCount,
		
		ReadmeFn: readmeFn,
		ReadmeData: readmeData,
		
		Imports: pdoc.Imports,
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
	Id string
	Packages []string
}

func CrawlPerson(httpClient *http.Client, id string) (*Person, error) {
	site, username := ParsePersonId(id)
	switch site {
	case "github.com":
		p, err := doc.GetGithubPerson(httpClient, map[string]string{"owner": username})
		if err != nil {
			return nil, err
		} else {
			return &Person{
				Id: id,
				Packages: p.Projects,
			}, nil
		}
	case "bitbucket.org":
		p, err := doc.GetBitbucketPerson(httpClient, map[string]string{"owner": username})
		if err != nil {
			return nil, err
		} else {
			return &Person{
				Id: id,
				Packages: p.Projects,
			}, nil
		}
	}
			
	return nil, nil
}

const pushPackageFieldName = "pkgjson"
const reportBadPackageFieldName = "pkg"

func PushPackage(httpClient *http.Client, urlStr string, p *Package) error {
	bytes, err := json.Marshal(p)
	if err != nil {
		return err
	}
	
	_, err = httpClient.PostForm(urlStr, url.Values{
		pushPackageFieldName: {string(bytes)},
	})
	
	return err
}

func ParsePushPackage(r *http.Request) (*Package, error) {
	jsonStr := r.FormValue(pushPackageFieldName)
	
	var p Package
	err := json.Unmarshal([]byte(jsonStr), &p)
	if err != nil {
		return nil, err
	}
	
	return &p, nil
}

func ReportBadPackage(httpClient *http.Client, urlStr, pkg string) error {
	_, err := httpClient.PostForm(urlStr, url.Values{
		reportBadPackageFieldName: {pkg},
	})
	return err
}

func ParseReportBadPackage(r *http.Request) (pkg string) {
	return r.FormValue(reportBadPackageFieldName)
}

const pushPersonFieldName = "psnjson"

type PushPersonReply struct {
	NewPackage bool
}

func PushPerson(httpClient *http.Client, urlStr string, p *Person) (*PushPersonReply, error) {
	bytes, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.PostForm(urlStr, url.Values{
		pushPersonFieldName: {string(bytes)},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var reply PushPersonReply
	
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&reply)
	if err != nil {
		return nil, err
	}
	
	return &reply, nil
}

func ParsePushPerson(r *http.Request) (*Person, error) {
	jsonStr := r.FormValue(pushPersonFieldName)
	
	var p Person
	err := json.Unmarshal([]byte(jsonStr), &p)
	if err != nil {
		fmt.Println("jsonStr:", jsonStr)
		return nil, err
	}
	
	return &p, nil
}
