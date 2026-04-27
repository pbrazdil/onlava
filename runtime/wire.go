package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	"pulse.dev/errs"
	"pulse.dev/internal/wire"
	"pulse.dev/runtime/shared"
)

type wireRecoveryRecord struct {
	StoredAt time.Time      `json:"stored_at"`
	Status   int            `json:"status"`
	Headers  map[string]any `json:"headers,omitempty"`
	Result   any            `json:"result,omitempty"`
	Error    any            `json:"error,omitempty"`
}

type wireRequest struct {
	SchemaHash string
	Method     string
	PathParams map[string]any
	Payload    any
}

func newWireRecoveryStore() map[string]wireRecoveryRecord {
	return make(map[string]wireRecoveryRecord)
}

func (s *server) registerWire() {
	s.public.Handle([]string{http.MethodGet}, wire.CapabilitiesPath, func(w http.ResponseWriter, req *http.Request, _ routeParams) {
		s.serveWireCapabilities(w)
	})
	s.public.Handle([]string{http.MethodGet}, wire.RecoverPathPrefix+":call_id", func(w http.ResponseWriter, req *http.Request, params routeParams) {
		s.serveWireRecovery(w, req, params.ByName("call_id"))
	})
	s.public.Handle([]string{http.MethodPost}, wire.CallPathPrefix+":endpoint_id", func(w http.ResponseWriter, req *http.Request, params routeParams) {
		s.serveWireCall(w, req, params.ByName("endpoint_id"))
	})
}

func (s *server) serveWireCapabilities(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(s.wireCapabilities())
}

func (s *server) wireCapabilities() wire.Capabilities {
	endpoints := listEndpoints()
	items := make([]wire.Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		item := wire.Endpoint{
			ID:                  ep.WireID,
			Service:             ep.Service,
			Endpoint:            ep.Name,
			Path:                ep.Path,
			Methods:             append([]string(nil), ep.Methods...),
			Available:           ep.WireAvailable,
			UnsupportedReason:   ep.WireUnsupportedReason,
			SchemaHash:          ep.WireSchemaHash,
			SafeJSONRetry:       wireMethodsSafe(ep.Methods),
			WirePath:            wire.CallPathPrefix + ep.WireID,
			RecoveryPathPattern: wire.RecoverPathPrefix + "{call_id}",
		}
		if item.ID == "" {
			item.ID = wire.EndpointID(ep.Service, ep.Name)
		}
		items = append(items, item)
	}
	return wire.NewCapabilities(wire.HashEndpoints(items), items)
}

func (s *server) serveWireRecovery(w http.ResponseWriter, req *http.Request, callID string) {
	callID = strings.TrimSpace(callID)
	if callID == "" {
		errs.HTTPError(w, errs.B().Code(errs.InvalidArgument).Msg("missing call id").Err())
		return
	}
	record, ok := s.lookupWireRecovery(callID)
	if !ok {
		errs.HTTPError(w, errs.B().Code(errs.NotFound).Msg("wire call result not found").Err())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(record.Status)
	_ = json.NewEncoder(w).Encode(record)
}

func (s *server) serveWireCall(w http.ResponseWriter, req *http.Request, endpointID string) {
	ep, ok := lookupEndpointByWireID(endpointID)
	if !ok || ep.Access == Private {
		s.writeWireFallback(w, errs.B().Code(errs.NotFound).Msg("wire endpoint not found").Err())
		return
	}
	if !ep.WireAvailable {
		msg := strings.TrimSpace(ep.WireUnsupportedReason)
		if msg == "" {
			msg = "wire transport unavailable for endpoint"
		}
		s.writeWireFallback(w, errs.B().Code(errs.Unimplemented).Msg(msg).Err())
		return
	}

	wireReq, err := decodeWireRequest(req.Body)
	if err != nil {
		s.writeWireFallback(w, errs.B().Code(errs.InvalidArgument).Msgf("invalid wire request: %v", err).Err())
		return
	}
	if wireReq.SchemaHash != "" && ep.WireSchemaHash != "" && wireReq.SchemaHash != ep.WireSchemaHash {
		s.writeWireFallback(w, errs.B().Code(errs.FailedPrecondition).Msg("wire schema mismatch").Err())
		return
	}

	pathValues, pathParams, err := decodeWirePathParams(ep, wireReq.PathParams)
	if err != nil {
		s.writeWireFallback(w, err)
		return
	}
	payload, err := decodeWirePayload(wireReq.Payload, ep.PayloadType)
	if err != nil {
		s.writeWireFallback(w, err)
		return
	}

	method := strings.ToUpper(strings.TrimSpace(wireReq.Method))
	if method == "" {
		method = preferredRuntimeMethod(ep.Methods)
	}
	wireRequestForState := req.Clone(req.Context())
	wireRequestForState.Method = method
	wireRequestForState.URL.Path = renderWireRequestPath(ep.Path, pathParams)

	state := newExternalState(ep, wireRequestForState, pathParams, payload, AuthInfo{})
	ctx := withState(req.Context(), state)
	restore := enterState(state)
	defer restore()
	startRequestTrace(state)

	authInfo, err := authenticateRequest(wireRequestForState.WithContext(ctx), ep)
	if err != nil {
		logRequestStart(state)
		finishRequestTrace(state, errs.HTTPStatus(err), nil, err)
		s.writeWireAppError(w, req, err, errs.HTTPStatus(err))
		return
	}
	state.auth = authInfo
	logRequestStart(state)

	resp, status, headers, callErr := executeTypedEndpoint(ep, ctx, pathValues, payload)
	applyHeaders(w.Header(), headers)
	defer finishRequestTrace(state, status, resp, callErr)
	if callErr != nil {
		s.writeWireAppError(w, req, callErr, status)
		return
	}

	body, bodyStatus, err := wireResponseBody(resp, w.Header())
	if err != nil {
		errs.HTTPError(w, errs.Wrap(err, "encode wire response"))
		return
	}
	if status == 0 {
		status = bodyStatus
	}
	if status == 0 {
		status = http.StatusOK
	}
	s.writeWireSuccess(w, req, status, body)
}

func decodeWireRequest(body io.Reader) (wireRequest, error) {
	if body == nil {
		return wireRequest{}, nil
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return wireRequest{}, err
	}
	value, err := wire.Decode(bytes.TrimSpace(data))
	if err != nil {
		return wireRequest{}, err
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return wireRequest{}, fmt.Errorf("request envelope must be an object")
	}
	req := wireRequest{
		SchemaHash: stringValue(obj["schema_hash"]),
		Method:     stringValue(obj["method"]),
		PathParams: objectValue(obj["path_params"]),
		Payload:    obj["payload"],
	}
	return req, nil
}

func decodeWirePathParams(ep *Endpoint, raw map[string]any) ([]any, shared.PathParams, error) {
	values := make([]any, 0, len(ep.PathParams))
	decoded := make(shared.PathParams, 0, len(ep.PathParams))
	for _, spec := range ep.PathParams {
		rawValue, ok := raw[spec.Name]
		if !ok {
			return nil, nil, errs.B().Code(errs.InvalidArgument).Msgf("missing path param %q", spec.Name).Err()
		}
		asString := fmt.Sprint(rawValue)
		value, err := decodeScalar(spec.Kind, asString)
		if err != nil {
			return nil, nil, errs.B().Code(errs.InvalidArgument).Msgf("invalid path param %q: %v", spec.Name, err).Err()
		}
		values = append(values, value)
		decoded = append(decoded, shared.PathParam{Name: spec.Name, Value: asString})
	}
	return values, decoded, nil
}

func decodeWirePayload(payload any, typ reflect.Type) (any, error) {
	if typ == nil {
		return nil, nil
	}
	target := newValueForType(typ)
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, errs.Wrap(err, "decode wire payload")
	}
	if len(bytes.TrimSpace(data)) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return finalizeValue(target, typ), nil
	}
	if err := json.Unmarshal(data, target.Interface()); err != nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msgf("invalid wire payload: %v", err).Err()
	}
	if err := maybeValidate(target.Interface()); err != nil {
		return nil, err
	}
	return finalizeValue(target, typ), nil
}

func wireResponseBody(resp any, headers http.Header) (any, int, error) {
	if resp == nil {
		return nil, 0, nil
	}
	value := reflect.ValueOf(resp)
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return nil, 0, nil
	}
	if isStructLike(value.Type()) {
		return splitResponse(resp, headers)
	}
	return resp, 0, nil
}

func (s *server) writeWireSuccess(w http.ResponseWriter, req *http.Request, status int, result any) {
	envelope := map[string]any{
		"ok":     true,
		"status": status,
		"result": result,
	}
	s.storeWireRecovery(req, status, result, nil)
	data, err := wire.Encode(envelope)
	if err != nil {
		errs.HTTPError(w, errs.Wrap(err, "encode wire response"))
		return
	}
	w.Header().Set("Content-Type", wire.ContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(wire.SchemaHashHeader, s.wireCapabilities().SchemaHash)
	if callID := strings.TrimSpace(req.Header.Get(wire.CallIDHeader)); callID != "" {
		w.Header().Set(wire.CallIDHeader, callID)
	}
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func (s *server) writeWireAppError(w http.ResponseWriter, req *http.Request, err error, status int) {
	if status == 0 {
		status = errs.HTTPStatus(err)
	}
	payload := errorPayload(err)
	envelope := map[string]any{
		"ok":     false,
		"status": status,
		"error":  payload,
	}
	s.storeWireRecovery(req, status, nil, payload)
	data, encodeErr := wire.Encode(envelope)
	if encodeErr != nil {
		errs.HTTPErrorWithCode(w, err, status)
		return
	}
	w.Header().Set("Content-Type", wire.ContentType)
	w.Header().Set("Cache-Control", "no-store")
	if callID := strings.TrimSpace(req.Header.Get(wire.CallIDHeader)); callID != "" {
		w.Header().Set(wire.CallIDHeader, callID)
	}
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func (s *server) writeWireFallback(w http.ResponseWriter, err error) {
	w.Header().Set(wire.FallbackHeader, "json")
	errs.HTTPError(w, err)
}

func (s *server) storeWireRecovery(req *http.Request, status int, result any, errPayload any) {
	callID := strings.TrimSpace(req.Header.Get(wire.CallIDHeader))
	if callID == "" {
		return
	}
	record := wireRecoveryRecord{
		StoredAt: time.Now().UTC(),
		Status:   status,
		Result:   result,
		Error:    errPayload,
	}
	s.wireRecoveryMu.Lock()
	if s.wireRecovery == nil {
		s.wireRecovery = newWireRecoveryStore()
	}
	s.wireRecovery[callID] = record
	s.pruneWireRecoveryLocked(time.Now().Add(-10 * time.Minute))
	s.wireRecoveryMu.Unlock()
}

func (s *server) lookupWireRecovery(callID string) (wireRecoveryRecord, bool) {
	s.wireRecoveryMu.Lock()
	defer s.wireRecoveryMu.Unlock()
	s.pruneWireRecoveryLocked(time.Now().Add(-10 * time.Minute))
	record, ok := s.wireRecovery[callID]
	return record, ok
}

func (s *server) pruneWireRecoveryLocked(before time.Time) {
	for callID, record := range s.wireRecovery {
		if record.StoredAt.Before(before) {
			delete(s.wireRecovery, callID)
		}
	}
}

func lookupEndpointByWireID(id string) (*Endpoint, bool) {
	id = strings.TrimSpace(id)
	for _, ep := range listEndpoints() {
		wireID := ep.WireID
		if wireID == "" {
			wireID = wire.EndpointID(ep.Service, ep.Name)
		}
		if wireID == id {
			return ep, true
		}
	}
	return nil, false
}

func errorPayload(err error) map[string]any {
	payload := map[string]any{
		"code":    string(errs.Code(err)),
		"message": "",
	}
	if err != nil {
		payload["message"] = err.Error()
	}
	if details := errs.Details(err); details != nil {
		payload["details"] = details
	}
	if meta := errs.Meta(err); meta != nil {
		payload["meta"] = meta
	}
	return payload
}

func renderWireRequestPath(pattern string, params shared.PathParams) string {
	path := pattern
	for _, param := range params {
		path = strings.ReplaceAll(path, ":"+param.Name, param.Value)
		path = strings.ReplaceAll(path, "*"+param.Name, param.Value)
	}
	return path
}

func preferredRuntimeMethod(methods []string) string {
	for _, method := range methods {
		if strings.EqualFold(method, http.MethodGet) {
			return http.MethodGet
		}
	}
	if len(methods) > 0 {
		return strings.ToUpper(methods[0])
	}
	return http.MethodPost
}

func wireMethodsSafe(methods []string) bool {
	if len(methods) == 0 {
		return false
	}
	for _, method := range methods {
		if method == "*" {
			return false
		}
		if !wire.IsSafeMethod(method) {
			return false
		}
	}
	return true
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func objectValue(value any) map[string]any {
	if value == nil {
		return nil
	}
	if obj, ok := value.(map[string]any); ok {
		return obj
	}
	return nil
}

func listWireEndpointIDs() []string {
	endpoints := listEndpoints()
	ids := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep.WireAvailable {
			ids = append(ids, ep.WireID)
		}
	}
	slices.Sort(ids)
	return ids
}
