package runtime

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

var (
	secretsEnvOnce sync.Once
	secretsEnvData map[string]string
	secretsEnvErr  error
)

func MustPopulateSecrets(target any) {
	if err := PopulateSecrets(target); err != nil {
		panic(err)
	}
}

func PopulateSecrets(target any) error {
	value := reflect.ValueOf(target)
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return fmt.Errorf("runtime: secrets target must be a non-nil pointer to struct")
	}
	elem := value.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("runtime: secrets target must point to struct, got %s", elem.Kind())
	}

	env, err := loadSecretsEnv()
	if err != nil {
		return err
	}

	typ := elem.Type()
	var missing []missingSecret
	for i := 0; i < elem.NumField(); i++ {
		field := elem.Field(i)
		structField := typ.Field(i)
		if !structField.IsExported() {
			continue
		}
		if field.Kind() != reflect.String {
			return fmt.Errorf("runtime: secret field %s must be string, got %s", structField.Name, field.Type())
		}
		keys := secretEnvKeys(structField.Name)
		value, ok := lookupSecretValue(env, keys)
		if !ok {
			missing = append(missing, missingSecret{Field: structField.Name, Keys: keys})
			continue
		}
		field.SetString(value)
	}
	logMissingSecrets(missing)
	return nil
}

type missingSecret struct {
	Field string
	Keys  []string
}

func logMissingSecrets(missing []missingSecret) {
	if len(missing) == 0 {
		return
	}
	fields := make([]string, 0, len(missing))
	keys := make([]string, 0, len(missing)*2)
	seenKeys := make(map[string]bool, len(missing)*2)
	for _, secret := range missing {
		fields = append(fields, secret.Field)
		for _, key := range secret.Keys {
			if seenKeys[key] {
				continue
			}
			seenKeys[key] = true
			keys = append(keys, key)
		}
	}
	slices.Sort(fields)
	slices.Sort(keys)
	slog.Warn("pulse secrets missing", "fields", fields, "env_keys", keys, "source", ".env")
}

func loadSecretsEnv() (map[string]string, error) {
	secretsEnvOnce.Do(func() {
		secretsEnvData, secretsEnvErr = parseDotEnv(".env")
	})
	return secretsEnvData, secretsEnvErr
}

func parseDotEnv(path string) (map[string]string, error) {
	data := make(map[string]string)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return data, nil
	}
	if err != nil {
		return nil, fmt.Errorf("runtime: read %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if lineNo == 1 {
			line = strings.TrimPrefix(line, "\uFEFF")
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("runtime: invalid .env line %d", lineNo)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("runtime: invalid empty .env key on line %d", lineNo)
		}
		value, err := parseDotEnvValue(strings.TrimSpace(rawValue))
		if err != nil {
			return nil, fmt.Errorf("runtime: parse .env line %d: %w", lineNo, err)
		}
		data[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("runtime: scan %s: %w", path, err)
	}
	return data, nil
}

func parseDotEnvValue(value string) (string, error) {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", err
		}
		return unquoted, nil
	}
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1], nil
	}
	return value, nil
}

func lookupSecretValue(fileEnv map[string]string, keys []string) (string, bool) {
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			return value, true
		}
	}
	for _, key := range keys {
		if value, ok := fileEnv[key]; ok {
			return value, true
		}
	}
	return "", false
}

func secretEnvKeys(fieldName string) []string {
	keys := []string{fieldName}
	alt := toEnvKey(fieldName)
	if alt != "" && alt != fieldName {
		keys = append(keys, alt)
	}
	return keys
}

func toEnvKey(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		if i > 0 && shouldInsertUnderscore(runes[i-1], r, nextRune(runes, i)) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

func nextRune(runes []rune, index int) rune {
	if index+1 >= len(runes) {
		return 0
	}
	return runes[index+1]
}

func shouldInsertUnderscore(prev, current, next rune) bool {
	if !unicode.IsUpper(current) {
		return false
	}
	if unicode.IsLower(prev) || unicode.IsDigit(prev) {
		return true
	}
	return unicode.IsUpper(prev) && next != 0 && unicode.IsLower(next)
}
