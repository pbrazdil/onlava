// Package data exposes onlava's native dynamic data platform for app code.
package data

import (
	"context"
	"net/http"

	onlavaauth "github.com/pbrazdil/onlava/auth"
	"github.com/pbrazdil/onlava/internal/datastore"
)

type (
	DB                   = datastore.DB
	Options              = datastore.Options
	Store                = datastore.Store
	Tenant               = datastore.Tenant
	Object               = datastore.Object
	Field                = datastore.Field
	FieldType            = datastore.FieldType
	PhysicalColumn       = datastore.PhysicalColumn
	FieldOption          = datastore.FieldOption
	FieldOptionRequest   = datastore.FieldOptionRequest
	Actor                = datastore.Actor
	CreateObjectRequest  = datastore.CreateObjectRequest
	CreateFieldRequest   = datastore.CreateFieldRequest
	CreateRecordRequest  = datastore.CreateRecordRequest
	UpdateRecordRequest  = datastore.UpdateRecordRequest
	DeleteRecordRequest  = datastore.DeleteRecordRequest
	QueryRecordsRequest  = datastore.QueryRecordsRequest
	Record               = datastore.Record
	RecordResponse       = datastore.RecordResponse
	DeleteRecordResponse = datastore.DeleteRecordResponse
	RecordPage           = datastore.RecordPage
	Query                = datastore.Query
	Filter               = datastore.Filter
	Sort                 = datastore.Sort
	SubscriptionRequest  = datastore.SubscriptionRequest
	Event                = datastore.Event
	Permissions          = datastore.Permissions
	AllowAllPermissions  = datastore.AllowAllPermissions
	ObjectRef            = datastore.ObjectRef
	FieldRef             = datastore.FieldRef
)

const (
	FieldText        = datastore.FieldText
	FieldRichText    = datastore.FieldRichText
	FieldNumber      = datastore.FieldNumber
	FieldNumeric     = datastore.FieldNumeric
	FieldCurrency    = datastore.FieldCurrency
	FieldBoolean     = datastore.FieldBoolean
	FieldDate        = datastore.FieldDate
	FieldDatetime    = datastore.FieldDatetime
	FieldUUID        = datastore.FieldUUID
	FieldSelect      = datastore.FieldSelect
	FieldMultiSelect = datastore.FieldMultiSelect
	FieldRating      = datastore.FieldRating
	FieldJSON        = datastore.FieldJSON
	FieldRawJSON     = datastore.FieldRawJSON
	FieldFiles       = datastore.FieldFiles
	FieldFullName    = datastore.FieldFullName
	FieldAddress     = datastore.FieldAddress
	FieldEmails      = datastore.FieldEmails
	FieldPhones      = datastore.FieldPhones
	FieldRelation    = datastore.FieldRelation
)

func Open(ctx context.Context, db DB, opts Options) (*Store, error) {
	return datastore.Open(ctx, db, opts)
}

func ActorFromContext(context.Context) Actor {
	var actor Actor
	if uid, ok := onlavaauth.UserID(); ok {
		actor.ID = string(uid)
	}
	if data := onlavaauth.Data(); data != nil {
		actor.Data = data
	}
	return actor
}

func ServeEvents(ctx context.Context, store *Store, actor Actor, w http.ResponseWriter, req *http.Request) error {
	return store.ServeEvents(ctx, actor, w, req)
}
