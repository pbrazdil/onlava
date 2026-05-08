package datastore

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

const maxIdentifierLength = 63

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

var reservedIdentifiers = map[string]bool{
	"all": true, "and": true, "as": true, "by": true, "case": true, "create": true,
	"delete": true, "desc": true, "drop": true, "false": true, "from": true,
	"group": true, "in": true, "insert": true, "is": true, "join": true,
	"limit": true, "not": true, "null": true, "or": true, "order": true,
	"select": true, "table": true, "true": true, "update": true, "where": true,
}

func validateName(kind, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%s name is required", kind)
	}
	if len(name) > maxIdentifierLength {
		return fmt.Errorf("%s name %q is longer than %d characters", kind, name, maxIdentifierLength)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("%s name %q must start with a lowercase letter and contain only lowercase letters, digits, and underscores", kind, name)
	}
	if reservedIdentifiers[name] {
		return fmt.Errorf("%s name %q is reserved", kind, name)
	}
	return nil
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func qualifiedIdent(schema, name string) string {
	return quoteIdent(schema) + "." + quoteIdent(name)
}

func safeColumnName(field string, suffix string) string {
	name := field
	if suffix != "" {
		name += "_" + suffix
	}
	if len(name) <= maxIdentifierLength {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	prefix := name[:maxIdentifierLength-9]
	return strings.TrimRight(prefix, "_") + "_" + hex.EncodeToString(sum[:])[:8]
}

func physicalTableName(tenantID, objectName string) string {
	sum := sha256.Sum256([]byte(tenantID + ":" + objectName))
	prefix := "t_" + hex.EncodeToString(sum[:])[:12] + "_"
	remaining := maxIdentifierLength - len(prefix)
	if len(objectName) > remaining {
		objectName = objectName[:remaining]
		objectName = strings.TrimRight(objectName, "_")
	}
	return prefix + objectName
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func advisoryLockKey(parts ...string) int64 {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

func defaultLabel(name string) string {
	parts := strings.Split(name, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}
