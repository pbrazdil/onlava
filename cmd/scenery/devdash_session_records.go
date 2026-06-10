package main

import (
	"context"
	"database/sql"

	"scenery.sh/internal/devdash"
)

func devdashAppRecordForSession(ctx context.Context, store *devdash.Store, appID, sessionID string) (devdash.AppRecord, bool, error) {
	if sessionID != "" {
		record, err := store.GetAppForSession(ctx, appID, sessionID)
		if err == nil {
			return record, true, nil
		}
		if err != sql.ErrNoRows {
			return devdash.AppRecord{}, false, err
		}
	}
	record, err := store.GetApp(ctx, appID)
	if err != nil {
		return devdash.AppRecord{}, false, err
	}
	return record, false, nil
}
