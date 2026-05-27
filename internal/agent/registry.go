package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Registry struct {
	path       string
	router     string
	scheme     string
	mu         sync.Mutex
	sessions   map[string]Session
	substrates map[string]Substrate
}

type registryFile struct {
	Sessions   []Session   `json:"sessions"`
	Substrates []Substrate `json:"substrates,omitempty"`
}

func OpenRegistry(path, routerAddr string, routerScheme ...string) (*Registry, error) {
	scheme := "http"
	if len(routerScheme) > 0 && strings.TrimSpace(routerScheme[0]) != "" {
		scheme = strings.TrimSpace(routerScheme[0])
	}
	r := &Registry{
		path:       path,
		router:     routerAddr,
		scheme:     scheme,
		sessions:   make(map[string]Session),
		substrates: make(map[string]Substrate),
	}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Registry) UpsertSubstrate(req UpsertSubstrateRequest) (Substrate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kind := sanitizeLabel(req.Kind)
	if kind == "" {
		return Substrate{}, errors.New("substrate kind must not be empty")
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "ready"
	}
	now := time.Now().UTC()
	createdAt := now
	if current, ok := r.substrates[kind]; ok && !current.CreatedAt.IsZero() {
		createdAt = current.CreatedAt
	}
	substrate := Substrate{
		SchemaVersion: SubstrateSchemaVersion,
		Kind:          kind,
		Status:        status,
		OwnerPID:      req.OwnerPID,
		PIDs:          copyIntMap(req.PIDs),
		URLs:          copyStringMap(req.URLs),
		Endpoints:     copyStringMap(req.Endpoints),
		CreatedAt:     createdAt,
		UpdatedAt:     now,
	}
	r.substrates[kind] = substrate
	if err := r.saveLocked(); err != nil {
		return Substrate{}, err
	}
	return substrate, nil
}

func (r *Registry) GetSubstrate(kind string) (Substrate, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	substrate, ok := r.substrates[sanitizeLabel(kind)]
	return substrate, ok
}

func (r *Registry) ListSubstrates() []Substrate {
	r.mu.Lock()
	defer r.mu.Unlock()
	return sortedSubstrates(r.substrates)
}

func (r *Registry) DeleteSubstrate(kind string) (Substrate, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := sanitizeLabel(kind)
	substrate, ok := r.substrates[key]
	if !ok {
		return Substrate{}, false, nil
	}
	delete(r.substrates, key)
	if err := r.saveLocked(); err != nil {
		return Substrate{}, false, err
	}
	return substrate, true, nil
}

func (r *Registry) Upsert(req RegisterRequest) (Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sessionID := SessionID(req.AppRoot, req.Branch)
	var existing *Session
	if current, ok := r.sessions[sessionID]; ok {
		existing = &current
	}
	session, err := NewSession(req, r.router, r.scheme, existing)
	if err != nil {
		return Session{}, err
	}
	r.sessions[session.SessionID] = session
	if err := r.saveLocked(); err != nil {
		return Session{}, err
	}
	if err := WriteManifest(session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (r *Registry) Get(id string) (Session, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[strings.TrimSpace(id)]
	return session, ok
}

func (r *Registry) List() []Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	return sortedSessions(r.sessions)
}

func (r *Registry) FindByAppRoot(root string) []Session {
	root = filepath.Clean(strings.TrimSpace(root))
	r.mu.Lock()
	defer r.mu.Unlock()
	var matches []Session
	for _, session := range r.sessions {
		if filepath.Clean(session.AppRoot) == root {
			matches = append(matches, session)
		}
	}
	sortSessions(matches)
	return matches
}

func (r *Registry) Delete(id string) (Session, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[strings.TrimSpace(id)]
	if !ok {
		return Session{}, false, nil
	}
	delete(r.sessions, id)
	if err := r.saveLocked(); err != nil {
		return Session{}, false, err
	}
	return session, true, nil
}

func (r *Registry) load() error {
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var file registryFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}
	for _, session := range file.Sessions {
		if session.SessionID == "" {
			continue
		}
		r.sessions[session.SessionID] = session
	}
	for _, substrate := range file.Substrates {
		kind := sanitizeLabel(substrate.Kind)
		if kind == "" {
			continue
		}
		substrate.Kind = kind
		r.substrates[kind] = substrate
	}
	return nil
}

func (r *Registry) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(registryFile{
		Sessions:   sortedSessions(r.sessions),
		Substrates: sortedSubstrates(r.substrates),
	}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWriteFile(r.path, data, 0o644)
}

func sortedSubstrates(substrates map[string]Substrate) []Substrate {
	items := make([]Substrate, 0, len(substrates))
	for _, substrate := range substrates {
		items = append(items, substrate)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Kind < items[j].Kind
	})
	return items
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	copied := make(map[string]string, len(values))
	for key, value := range values {
		key = sanitizeLabel(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		copied[key] = value
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

func copyIntMap(values map[string]int) map[string]int {
	if len(values) == 0 {
		return nil
	}
	copied := make(map[string]int, len(values))
	for key, value := range values {
		key = sanitizeLabel(key)
		if key == "" || value <= 0 {
			continue
		}
		copied[key] = value
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

func sortedSessions(sessions map[string]Session) []Session {
	items := make([]Session, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, session)
	}
	sortSessions(items)
	return items
}

func sortSessions(items []Session) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Status == items[j].Status {
			return items[i].SessionID < items[j].SessionID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
