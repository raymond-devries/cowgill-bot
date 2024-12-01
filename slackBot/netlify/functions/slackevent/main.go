package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"net/http"
)

func main() {
	setRoutes()
	lambda.Start(httpadapter.New(http.DefaultServeMux).ProxyWithContext)
}
