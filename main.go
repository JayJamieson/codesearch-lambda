package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/go-git/go-git/v5"
	"github.com/google/codesearch/index"
	"github.com/google/codesearch/regexp"
)

var master = "/tmp/.csearchindex"

func debugHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	eventJson, _ := json.MarshalIndent(request, "", "  ")

	log.Printf("EVENT: %s", eventJson)

	// environment variables
	log.Printf("REGION: %s", os.Getenv("AWS_REGION"))
	log.Println("ALL ENV VARS:")
	for _, element := range os.Environ() {
		log.Println(element)
	}

	// request context
	lc, _ := lambdacontext.FromContext(ctx)
	log.Printf("REQUEST ID: %s", lc.AwsRequestID)

	// global variable
	log.Printf("FUNCTION NAME: %s", lambdacontext.FunctionName)

	// context method
	deadline, _ := ctx.Deadline()
	log.Printf("DEADLINE: %s", deadline)

	return events.APIGatewayV2HTTPResponse{Body: "Success", StatusCode: 200}, nil
}

func handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {

	var err error
	var result string
	switch request.RequestContext.HTTP.Path {
	case "/cindex":
		err = indexRepo(ctx, request)
		result = "Success"
	case "/csearch":
		result, err = searchRepo(ctx, request)
	default:
		return events.APIGatewayV2HTTPResponse{Body: "Not Found", StatusCode: 404}, nil
	}

	// spew.Dump(request)

	if err != nil {
		log.Printf("Error: %s", err)
		return events.APIGatewayV2HTTPResponse{Body: err.Error(), StatusCode: 500}, err
	}

	return events.APIGatewayV2HTTPResponse{Body: result, StatusCode: 200}, nil
}

func searchRepo(_ context.Context, request events.APIGatewayV2HTTPRequest) (string, error) {
	term, tOk := request.QueryStringParameters["q"]
	flags, fOk := request.QueryStringParameters["args"]

	if !tOk || term == "" || !fOk || flags == "" {
		return "", errors.New("args or q not provided")
	}

	if request.RequestContext.HTTP.Method != "GET" {
		return "", errors.New("method not allowed")
	}

	commandLine := flag.NewFlagSet("", flag.ExitOnError)
	var fFlag = commandLine.String("f", "", "search only files with names matching this regexp")
	var iFlag = commandLine.Bool("i", false, "case-insensitive search")
	var htmlFlag = commandLine.Bool("html", false, "print HTML output")
	var verboseFlag = commandLine.Bool("verbose", false, "print extra information")
	var bruteFlag = commandLine.Bool("brute", false, "brute force - search all files in index")
	var cpuProfile = commandLine.String("cpuprofile", "", "write cpu profile to this file")

	log.SetPrefix("csearch: ")
	output := bytes.NewBufferString("")
	g := regexp.Grep{
		Stdout:  output,
		Stderr:  output,
		FlagSet: commandLine,
	}

	g.AddFlags()

	commandLine.Usage = func() {
		fmt.Fprintf(output, "Usage: csearch [flags] <regexp>\n")
	}

	commandLine.Parse(append(strings.Split(flags, ","), term))

	if *htmlFlag {
		g.HTML = true
	}
	args := commandLine.Args()

	if len(args) != 1 {
		return "", errors.New("args not provided")
	}

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	pat := "(?m)" + args[0]
	if *iFlag {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		log.Fatal(err)
	}
	g.Regexp = re
	var fre *regexp.Regexp
	if *fFlag != "" {
		fre, err = regexp.Compile(*fFlag)
		if err != nil {
			log.Fatal(err)
		}
	}
	q := index.RegexpQuery(re.Syntax)
	if *verboseFlag {
		log.Printf("query: %s\n", q)
	}

	ix := index.Open(master)
	ix.Verbose = *verboseFlag
	var post []int
	if *bruteFlag {
		post = ix.PostingQuery(&index.Query{Op: index.QAll})
	} else {
		post = ix.PostingQuery(q)
	}
	if *verboseFlag {
		log.Printf("post query identified %d possible files\n", len(post))
	}

	if fre != nil {
		fnames := make([]int, 0, len(post))

		for _, fileid := range post {
			name := ix.Name(fileid)
			if fre.MatchString(name.String(), true, true) < 0 {
				continue
			}
			fnames = append(fnames, fileid)
		}

		if *verboseFlag {
			log.Printf("filename regexp matched %d files\n", len(fnames))
		}
		post = fnames
	}

	var (
		zipFile   string
		zipReader *zip.ReadCloser
		zipMap    map[string]*zip.File
	)

	for _, fileid := range post {
		name := ix.Name(fileid).String()
		if g.L && (pat == "(?m)" || pat == "(?i)(?m)") {
			g.Reader(bytes.NewReader(nil), name)
			continue
		}
		file, err := os.Open(string(name))
		if err != nil {
			if i := strings.Index(name, ".zip\x01"); i >= 0 {
				zfile, zname := name[:i+4], name[i+5:]
				if zfile != zipFile {
					if zipReader != nil {
						zipReader.Close()
						zipMap = nil
					}
					zipFile = zfile
					zipReader, err = zip.OpenReader(zfile)
					if err != nil {
						zipReader = nil
					}
					if zipReader != nil {
						zipMap = make(map[string]*zip.File)
						for _, file := range zipReader.File {
							zipMap[file.Name] = file
						}
					}
				}
				file := zipMap[zname]
				if file != nil {
					r, err := file.Open()
					if err != nil {
						continue
					}
					g.Reader(r, name)
					r.Close()
					continue
				}
			}
			continue
		}
		g.Reader(file, name)
		file.Close()
	}

	if !g.Match {
		return "", errors.New("no matches found")
	}

	return output.String(), nil
}

func indexRepo(_ context.Context, request events.APIGatewayV2HTTPRequest) error {
	if request.RequestContext.HTTP.Method != "POST" {
		return errors.New("method not allowed")
	}
	// Get the body of the request
	body := request.Body

	// Unmarshal the body into a struct
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return err
	}

	repoPath, ok := data["repo"].(string)

	if !ok {
		fmt.Printf("%+v\n", data)
		return errors.New("repo not found")
	}

	repoURL, _ := url.Parse(repoPath)
	clonePath := "/tmp" + repoURL.Path

	_, err := git.PlainClone(clonePath, false, &git.CloneOptions{
		URL:      repoPath,
		Progress: os.Stdout,
	})

	if err != nil {
		return err
	}
	var roots []index.Path = []index.Path{index.MakePath(clonePath)}
	resetFlag := false
	checkFlag := false

	if _, err := os.Stat(master); err != nil {
		// Does not exist.
		resetFlag = true
	}
	file := master
	if !resetFlag {
		file += "~"
		if checkFlag {
			ix := index.Open(master)
			if err := ix.Check(); err != nil {
				log.Fatal(err)
			}
		}
	}

	ix := index.Create(file)
	ix.Verbose = false
	ix.Zip = false
	ix.AddRoots(roots)
	for _, root := range roots {
		log.Printf("index %s", root)
		filepath.Walk(root.String(), func(path string, info os.FileInfo, err error) error {
			if _, elem := filepath.Split(path); elem != "" {
				// Skip various temporary or "hidden" files or directories.
				if elem[0] == '.' || elem[0] == '#' || elem[0] == '~' || elem[len(elem)-1] == '~' {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if err != nil {
				log.Printf("%s: %s", path, err)
				return nil
			}
			if info != nil && info.Mode()&os.ModeType == 0 {
				if err := ix.AddFile(path); err != nil {
					log.Printf("%s: %s", path, err)
					return nil
				}
			}
			return nil
		})
	}
	log.Printf("flush index")
	ix.Flush()

	if !resetFlag {
		log.Printf("merge %s %s", master, file)
		index.Merge(file+"~", master, file)
		if checkFlag {
			ix := index.Open(file + "~")
			if err := ix.Check(); err != nil {
				log.Fatal(err)
			}
		}
		os.Remove(file)
		os.Rename(file+"~", master)
	} else {
		if checkFlag {
			ix := index.Open(file)
			if err := ix.Check(); err != nil {
				log.Fatal(err)
			}
		}
	}

	log.Printf("done")

	ixs := index.Open(master)
	ixs.PrintStats()

	return nil
}

func main() {
	// https://docs.aws.amazon.com/lambda/latest/dg/golang-handler.html

	// Uncoment this to use the debug handler and comment out lambda.Start(handler)
	// lambda.Start(debugHandler)

	lambda.Start(handler)
}
