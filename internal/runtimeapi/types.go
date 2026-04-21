package runtimeapi

type Access string

const (
	Public  Access = "public"
	Auth    Access = "auth"
	Private Access = "private"
)

type ParamKind string

const (
	ParamString ParamKind = "string"
	ParamBool   ParamKind = "bool"
	ParamInt    ParamKind = "int"
	ParamInt8   ParamKind = "int8"
	ParamInt16  ParamKind = "int16"
	ParamInt32  ParamKind = "int32"
	ParamInt64  ParamKind = "int64"
	ParamUint   ParamKind = "uint"
	ParamUint8  ParamKind = "uint8"
	ParamUint16 ParamKind = "uint16"
	ParamUint32 ParamKind = "uint32"
	ParamUint64 ParamKind = "uint64"
)

type ParamSpec struct {
	Name string
	Kind ParamKind
}
