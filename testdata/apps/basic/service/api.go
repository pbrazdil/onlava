package service

import (
	"context"
	"encoding/json"
	"net/http"

	onlava "onlava.com"
	onlavaauth "onlava.com/auth"
	"onlava.com/errs"
)

//onlava:service
type Service struct {
	Prefix string
}

func initService() (*Service, error) {
	return &Service{Prefix: "hi"}, nil
}

type EchoRequest struct {
	Title  string `query:"title"`
	Header string `header:"X-Echo"`
	Body   string `json:"body"`
}

type EchoResponse struct {
	Message string `json:"message"`
}

//onlava:api public path=/echo/:name method=GET,POST
func (s *Service) Echo(ctx context.Context, name string, req *EchoRequest) (*EchoResponse, error) {
	return &EchoResponse{
		Message: s.Prefix + " " + name + " " + req.Title + " " + req.Header + " " + req.Body,
	}, nil
}

//onlava:api private
func (s *Service) Secret(ctx context.Context) (*EchoResponse, error) {
	return &EchoResponse{Message: "secret:" + s.Prefix}, nil
}

//onlava:api public
func (s *Service) CallPrivate(ctx context.Context) (*EchoResponse, error) {
	return s.Secret(ctx)
}

type AuthData struct {
	Role string `json:"role"`
}

//onlava:authhandler
func (s *Service) AuthHandler(ctx context.Context, token string) (onlavaauth.UID, *AuthData, error) {
	if token != "token123" {
		return "", nil, errs.B().Code(errs.Unauthenticated).Msg("bad token").Err()
	}
	return "user-1", &AuthData{Role: "admin"}, nil
}

type AuthEchoResponse struct {
	User string `json:"user"`
	Role string `json:"role"`
}

//onlava:api auth
func (s *Service) AuthEcho(ctx context.Context) (*AuthEchoResponse, error) {
	userID, ok := onlavaauth.UserID()
	if !ok {
		return nil, errs.B().Code(errs.Unauthenticated).Msg("missing auth").Err()
	}
	data := onlavaauth.Data().(*AuthData)
	return &AuthEchoResponse{User: string(userID), Role: data.Role}, nil
}

type StatusResponse struct {
	Message string `json:"message"`
	Status  int    `onlava:"httpstatus"`
}

//onlava:api public
func (s *Service) CustomStatus(ctx context.Context) (*StatusResponse, error) {
	return &StatusResponse{Message: "created", Status: 201}, nil
}

//onlava:api public raw path=/raw/*rest
func (s *Service) Raw(w http.ResponseWriter, req *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]string{
		"path":   onlava.CurrentRequest().PathParams.Get("rest"),
		"method": onlava.CurrentRequest().Method,
	})
}
