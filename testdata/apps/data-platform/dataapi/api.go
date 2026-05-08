package dataapi

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/pbrazdil/onlava/auth"
	"github.com/pbrazdil/onlava/data"
	"github.com/pbrazdil/onlava/errs"
	"github.com/pbrazdil/onlava/pgxpool"
)

//onlava:service
type Service struct {
	pool  *pgxpool.Pool
	store *data.Store
}

func initService() (*Service, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("ONLAVA_TEST_DATABASE_URL")
	}
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL or ONLAVA_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	store, err := data.Open(context.Background(), pool, data.Options{})
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &Service{pool: pool, store: store}, nil
}

func (s *Service) Shutdown(context.Context) {
	if s.pool != nil {
		s.pool.Close()
	}
}

type AuthData struct {
	Role string `json:"role"`
}

//onlava:authhandler
func (s *Service) AuthHandler(ctx context.Context, token string) (auth.UID, *AuthData, error) {
	if token != "token123" {
		return "", nil, errs.B().Code(errs.Unauthenticated).Msg("bad token").Err()
	}
	return "fixture-user", &AuthData{Role: "tester"}, nil
}

//onlava:api auth path=/data/objects method=POST
func (s *Service) CreateObject(ctx context.Context, req *data.CreateObjectRequest) (*data.Object, error) {
	if req == nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("request is required").Err()
	}
	return s.store.CreateObject(ctx, data.ActorFromContext(ctx), *req)
}

//onlava:api auth path=/data/objects/:object/fields method=POST
func (s *Service) CreateField(ctx context.Context, object string, req *data.CreateFieldRequest) (*data.Field, error) {
	if req == nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("request is required").Err()
	}
	return s.store.CreateField(ctx, data.ActorFromContext(ctx), object, *req)
}

type EnableOutboxTriggersRequest struct {
	TenantKey string `json:"tenant_key"`
}

//onlava:api auth path=/data/objects/:object/outbox-triggers method=POST
func (s *Service) EnableOutboxTriggers(ctx context.Context, object string, req *EnableOutboxTriggersRequest) (*data.Object, error) {
	if req == nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("request is required").Err()
	}
	return s.store.EnableOutboxTriggers(ctx, data.ActorFromContext(ctx), req.TenantKey, object)
}

//onlava:api auth path=/data/objects/:object/records/query method=POST
func (s *Service) QueryRecords(ctx context.Context, object string, req *data.QueryRecordsRequest) (*data.RecordPage, error) {
	if req == nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("request is required").Err()
	}
	return s.store.QueryRecords(ctx, data.ActorFromContext(ctx), object, *req)
}

//onlava:api auth path=/data/objects/:object/records method=POST
func (s *Service) CreateRecord(ctx context.Context, object string, req *data.CreateRecordRequest) (*data.RecordResponse, error) {
	if req == nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("request is required").Err()
	}
	return s.store.CreateRecord(ctx, data.ActorFromContext(ctx), object, *req)
}

//onlava:api auth path=/data/objects/:object/records/:id method=PATCH
func (s *Service) UpdateRecord(ctx context.Context, object string, id string, req *data.UpdateRecordRequest) (*data.RecordResponse, error) {
	if req == nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("request is required").Err()
	}
	return s.store.UpdateRecord(ctx, data.ActorFromContext(ctx), object, id, *req)
}

//onlava:api auth path=/data/objects/:object/records/:id method=DELETE
func (s *Service) DeleteRecord(ctx context.Context, object string, id string, req *data.DeleteRecordRequest) (*data.DeleteRecordResponse, error) {
	if req == nil {
		req = &data.DeleteRecordRequest{}
	}
	return s.store.DeleteRecord(ctx, data.ActorFromContext(ctx), object, id, *req)
}

//onlava:api auth raw path=/data/events method=GET
func (s *Service) Events(w http.ResponseWriter, r *http.Request) {
	_ = s.store.ServeEvents(r.Context(), data.ActorFromContext(r.Context()), w, r)
}
