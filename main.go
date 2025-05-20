package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/go-git/go-git/v5"
)

func debugHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
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

	return events.APIGatewayProxyResponse{Body: "Success", StatusCode: 200}, nil
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	_, err := git.PlainClone("/tmp/go-git", false, &git.CloneOptions{
		URL:      "https://github.com/go-git/go-git",
		Progress: os.Stdout,
	})

	if err != nil {
		return events.APIGatewayProxyResponse{Body: "Failure", StatusCode: 500}, err
	}

	return events.APIGatewayProxyResponse{Body: "Success", StatusCode: 200}, nil
}

func main() {
	// https://docs.aws.amazon.com/lambda/latest/dg/golang-handler.html

	// Uncoment this to use the debug handler and comment out lambda.Start(handler)
	// lambda.Start(debugHandler)
	fmt.Printf("%v \n", os.Args[1:])
	os.Exit(0)
	lambda.Start(handler)
}
