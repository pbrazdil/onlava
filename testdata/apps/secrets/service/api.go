package service

import (
	"context"

	"example.com/secretsapp/helper"
)

var secrets struct {
	ServiceSecret string
}

type Response struct {
	Service string `json:"service"`
	Helper  string `json:"helper"`
}

//scenery:api public path=/secrets method=GET
func Get(ctx context.Context) (*Response, error) {
	return &Response{
		Service: secrets.ServiceSecret,
		Helper:  helper.Value(),
	}, nil
}
