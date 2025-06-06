# codesearch-lambda

Code Search Lambda is a FaaS wrapper around the original Code Search command line utility. It provides a HTTP API ontop of cindex and csearch command line applications.

## Building

In the root of project directory run:

`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o ./infrastructure/bootstrap main.go`

## Deploying

The output of `go build` command creates a `bootstrap` binary suitable for AWS Lambda environments.

Deployment is performed using terraform, run following commands:

1. `terraform init`
2. `terraform apply --auto-approve`

## Usage

TODO

## Original Code Search README

[Code Search](github.com/google/codesearch) is a tool for indexing and then performing
regular expression searches over large bodies of source code.
It is a set of command-line programs written in Go.

For background and an overview of the commands,
see http://swtch.com/~rsc/regexp/regexp4.html.

To install:

	go get github.com/google/codesearch/cmd/...

Use "go get -u" to update an existing installation.

Russ Cox
rsc@swtch.com
June 2015
