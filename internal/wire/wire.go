package wire

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

const (
	ContentType       = "application/vnd.pulse.wire+bin"
	CapabilitiesPath  = "/_wire/capabilities"
	CallPathPrefix    = "/_wire/"
	RecoverPathPrefix = "/_wire/recover/"
	CallIDHeader      = "X-Pulse-Call-ID"
	FallbackHeader    = "X-Pulse-Wire-Fallback"
	SchemaHashHeader  = "X-Pulse-Wire-Schema-Hash"
)

type Endpoint struct {
	ID                  string   `json:"id"`
	Service             string   `json:"service"`
	Endpoint            string   `json:"endpoint"`
	Path                string   `json:"path"`
	Methods             []string `json:"methods"`
	Available           bool     `json:"available"`
	UnsupportedReason   string   `json:"unsupported_reason,omitempty"`
	SchemaHash          string   `json:"schema_hash,omitempty"`
	SafeJSONRetry       bool     `json:"safe_json_retry"`
	WirePath            string   `json:"wire_path"`
	RecoveryPathPattern string   `json:"recovery_path_pattern"`
}

type Capabilities struct {
	SchemaVersion string              `json:"schema_version"`
	SchemaHash    string              `json:"wire_schema_hash"`
	ContentType   string              `json:"content_type"`
	Endpoints     map[string]Endpoint `json:"endpoints"`
}

func NewCapabilities(schemaHash string, endpoints []Endpoint) Capabilities {
	items := make(map[string]Endpoint, len(endpoints))
	for _, ep := range endpoints {
		if ep.ID == "" {
			continue
		}
		if ep.WirePath == "" {
			ep.WirePath = CallPathPrefix + ep.ID
		}
		if ep.RecoveryPathPattern == "" {
			ep.RecoveryPathPattern = RecoverPathPrefix + "{call_id}"
		}
		items[ep.ID] = ep
	}
	return Capabilities{
		SchemaVersion: "pulse.wire.capabilities.v1",
		SchemaHash:    schemaHash,
		ContentType:   ContentType,
		Endpoints:     items,
	}
}

func HashParts(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(strconv.Itoa(len(part))))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func HashEndpoints(endpoints []Endpoint) string {
	copyEndpoints := append([]Endpoint(nil), endpoints...)
	sort.Slice(copyEndpoints, func(i, j int) bool {
		return copyEndpoints[i].ID < copyEndpoints[j].ID
	})
	var parts []string
	for _, ep := range copyEndpoints {
		parts = append(parts,
			ep.ID,
			ep.Service,
			ep.Endpoint,
			ep.Path,
			strings.Join(ep.Methods, ","),
			strconv.FormatBool(ep.Available),
			ep.UnsupportedReason,
			ep.SchemaHash,
		)
	}
	return HashParts(parts...)
}

func EndpointID(service, endpoint string) string {
	return service + "." + endpoint
}

func IsSafeMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "GET", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}

const (
	tagNull byte = iota
	tagFalse
	tagTrue
	tagNumber
	tagString
	tagArray
	tagObject
)

var magic = []byte{'P', 'W', 'B', '1'}

func Encode(value any) ([]byte, error) {
	normalized, err := Normalize(value)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.Write(magic)
	if err := writeValue(&buf, normalized); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Decode(data []byte) (any, error) {
	if len(data) < len(magic) || !bytes.Equal(data[:len(magic)], magic) {
		return nil, fmt.Errorf("invalid pulse wire payload")
	}
	dec := decoder{data: data[len(magic):]}
	value, err := dec.readValue()
	if err != nil {
		return nil, err
	}
	if dec.pos != len(dec.data) {
		return nil, fmt.Errorf("trailing pulse wire data")
	}
	return value, nil
}

func Normalize(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var normalized any
	if err := dec.Decode(&normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func DecodeInto(data []byte, target any) error {
	value, err := Decode(data)
	if err != nil {
		return err
	}
	jsonData, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}

func writeValue(w io.Writer, value any) error {
	switch v := value.(type) {
	case nil:
		return writeByte(w, tagNull)
	case bool:
		if v {
			return writeByte(w, tagTrue)
		}
		return writeByte(w, tagFalse)
	case string:
		if err := writeByte(w, tagString); err != nil {
			return err
		}
		return writeBytes(w, []byte(v))
	case json.Number:
		n, err := v.Float64()
		if err != nil {
			return err
		}
		return writeNumber(w, n)
	case float64:
		return writeNumber(w, v)
	case float32:
		return writeNumber(w, float64(v))
	case int, int8, int16, int32, int64:
		return writeNumber(w, float64(reflect.ValueOf(v).Int()))
	case uint, uint8, uint16, uint32, uint64:
		return writeNumber(w, float64(reflect.ValueOf(v).Uint()))
	case []any:
		if err := writeByte(w, tagArray); err != nil {
			return err
		}
		if err := writeUvarint(w, uint64(len(v))); err != nil {
			return err
		}
		for _, item := range v {
			if err := writeValue(w, item); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		if err := writeByte(w, tagObject); err != nil {
			return err
		}
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if err := writeUvarint(w, uint64(len(keys))); err != nil {
			return err
		}
		for _, key := range keys {
			if err := writeBytes(w, []byte(key)); err != nil {
				return err
			}
			if err := writeValue(w, v[key]); err != nil {
				return err
			}
		}
		return nil
	default:
		normalized, err := Normalize(v)
		if err != nil {
			return err
		}
		return writeValue(w, normalized)
	}
}

func writeNumber(w io.Writer, value float64) error {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return fmt.Errorf("non-finite numbers are not supported")
	}
	if err := writeByte(w, tagNumber); err != nil {
		return err
	}
	var raw [8]byte
	binary.LittleEndian.PutUint64(raw[:], math.Float64bits(value))
	_, err := w.Write(raw[:])
	return err
}

func writeBytes(w io.Writer, data []byte) error {
	if err := writeUvarint(w, uint64(len(data))); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func writeUvarint(w io.Writer, value uint64) error {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], value)
	_, err := w.Write(buf[:n])
	return err
}

func writeByte(w io.Writer, value byte) error {
	_, err := w.Write([]byte{value})
	return err
}

type decoder struct {
	data []byte
	pos  int
}

func (d *decoder) readValue() (any, error) {
	tag, err := d.readByte()
	if err != nil {
		return nil, err
	}
	switch tag {
	case tagNull:
		return nil, nil
	case tagFalse:
		return false, nil
	case tagTrue:
		return true, nil
	case tagNumber:
		if d.pos+8 > len(d.data) {
			return nil, fmt.Errorf("truncated number")
		}
		raw := binary.LittleEndian.Uint64(d.data[d.pos : d.pos+8])
		d.pos += 8
		return math.Float64frombits(raw), nil
	case tagString:
		raw, err := d.readBytes()
		if err != nil {
			return nil, err
		}
		return string(raw), nil
	case tagArray:
		n, err := d.readUvarint()
		if err != nil {
			return nil, err
		}
		items := make([]any, 0, n)
		for i := uint64(0); i < n; i++ {
			item, err := d.readValue()
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	case tagObject:
		n, err := d.readUvarint()
		if err != nil {
			return nil, err
		}
		obj := make(map[string]any, n)
		for i := uint64(0); i < n; i++ {
			key, err := d.readBytes()
			if err != nil {
				return nil, err
			}
			value, err := d.readValue()
			if err != nil {
				return nil, err
			}
			obj[string(key)] = value
		}
		return obj, nil
	default:
		return nil, fmt.Errorf("unknown pulse wire tag %d", tag)
	}
}

func (d *decoder) readByte() (byte, error) {
	if d.pos >= len(d.data) {
		return 0, fmt.Errorf("truncated payload")
	}
	value := d.data[d.pos]
	d.pos++
	return value, nil
}

func (d *decoder) readBytes() ([]byte, error) {
	n, err := d.readUvarint()
	if err != nil {
		return nil, err
	}
	if n > uint64(len(d.data)-d.pos) {
		return nil, fmt.Errorf("truncated bytes")
	}
	value := d.data[d.pos : d.pos+int(n)]
	d.pos += int(n)
	return value, nil
}

func (d *decoder) readUvarint() (uint64, error) {
	value, n := binary.Uvarint(d.data[d.pos:])
	if n <= 0 {
		return 0, fmt.Errorf("invalid length")
	}
	d.pos += n
	return value, nil
}
