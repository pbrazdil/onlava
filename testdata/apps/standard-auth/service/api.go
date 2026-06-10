package service

import (
	"context"

	sceneryauth "scenery.sh/auth"
	"scenery.sh/errs"
)

type MeResponse struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
}

//scenery:api auth method=GET path=/whoami
func Whoami(ctx context.Context) (*MeResponse, error) {
	data, ok := sceneryauth.CurrentAuthData()
	if !ok {
		return nil, errs.B().Code(errs.Unauthenticated).Msg("missing auth").Err()
	}
	return &MeResponse{
		UserID:   string(data.UserID),
		TenantID: string(data.TenantID),
	}, nil
}
