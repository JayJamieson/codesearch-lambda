#!/bin/bash

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o ./infrastructure/bootstrap main.go

cd ./infrastructure

terraform apply --auto-approve

cd ..

echo "Done"
