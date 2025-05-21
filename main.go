package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/go-git/go-git/v5"
	"github.com/google/codesearch/index"
)

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
	switch request.RequestContext.HTTP.Path {
	case "/cindex":
		err = indexRepo(ctx, request)
	case "/csearch":
		return events.APIGatewayV2HTTPResponse{Body: "Not Found", StatusCode: 404}, nil
	default:
		return events.APIGatewayV2HTTPResponse{Body: "Not Found", StatusCode: 404}, nil
	}

	// spew.Dump(request)

	if err != nil {
		log.Printf("Error: %s", err)
		return events.APIGatewayV2HTTPResponse{Body: err.Error(), StatusCode: 500}, err
	}

	return events.APIGatewayV2HTTPResponse{Body: "Success", StatusCode: 200}, nil
}

func indexRepo(ctx context.Context, request events.APIGatewayV2HTTPRequest) error {
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

	master := "/tmp/.csearchindex"
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
