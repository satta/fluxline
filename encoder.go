package fluxline

// DCSO fluxline
// Copyright (c) 2017, 2018, DCSO GmbH

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/ShowMax/go-fqdn"
)

// Encoder represents a component that encapsulates a target environment for
// measurement submissions, as given by hostname and receiving writer.
type Encoder struct {
	host   string
	Writer io.Writer
}

func escapeSpecialChars(in string) string {
	str := strings.Replace(in, ",", `\,`, -1)
	str = strings.Replace(str, "=", `\=`, -1)
	str = strings.Replace(str, " ", `\ `, -1)
	return str
}

func toInfluxRepr(tag string, val interface{}) (string, error) {
	switch v := val.(type) {
	case string:
		if len(v) > 64000 {
			return "", fmt.Errorf("%s: string too long (%d characters, max. 64K)", tag, len(v))
		}
		return fmt.Sprintf("%q", v), nil
	case int32, int64, int16, int8, int:
		return fmt.Sprintf("%di", v), nil
	case uint32, uint64, uint16, uint8, uint:
		return fmt.Sprintf("%di", v), nil
	case float64, float32:
		return fmt.Sprintf("%g", v), nil
	case bool:
		return fmt.Sprintf("%t", v), nil
	case time.Time:
		return fmt.Sprintf("%d", uint64(v.UnixNano())), nil
	default:
		return "", fmt.Errorf("%s: unsupported type for Influx Line Protocol", tag)
	}
}

func recordFields(val interface{},
	fieldSet map[string]string) (map[string]string, error) {
	t := reflect.TypeOf(val)
	v := reflect.ValueOf(val)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("influx")
		if tag == "" {
			continue
		}
		repr, err := toInfluxRepr(tag, v.Field(i).Interface())
		if err != nil {
			return nil, err
		}
		fieldSet[tag] = repr
	}
	return fieldSet, nil
}

func (a *Encoder) formatLineProtocol(prefix string,
	tags map[string]string, fieldSet map[string]string) string {
	out := ""
	tagstr := ""

	// sort by key to obtain stable output order
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// serialize tags
	for _, k := range keys {
		tagstr += ","
		tagstr += fmt.Sprintf("%s=%s", escapeSpecialChars(k), escapeSpecialChars(tags[k]))
	}

	// sort by key to obtain stable output order
	keys = make([]string, 0, len(fieldSet))
	for key := range fieldSet {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// serialize fields
	first := true
	for _, k := range keys {
		if !first {
			out += ","
		} else {
			first = false
		}
		out += fmt.Sprintf("%s=%s", escapeSpecialChars(k), fieldSet[k])
	}
	if out == "" {
		return ""
	}

	// construct line protocol string
	return fmt.Sprintf("%s,host=%s%s %s %d\n", prefix, a.host,
		tagstr, out, uint64(time.Now().UnixNano()))
}

// Encode writes the line protocol representation for a given measurement
// name, data struct and tag map to the io.Writer specified on encoder creation.
func (a *Encoder) Encode(prefix string, val interface{},
	tags map[string]string) error {
	fieldSet := make(map[string]string)
	fieldSet, err := recordFields(val, fieldSet)
	if err != nil {
		return err
	}
	_, err = a.Writer.Write([]byte(a.formatLineProtocol(prefix, tags, fieldSet)))
	return err
}

// EncodeMap writes the line protocol representation for a given measurement
// name, field value map and tag map to the io.Writer specified on encoder
// creation.
func (a *Encoder) EncodeMap(prefix string, val map[string]string,
	tags map[string]string) error {
	_, err := a.Writer.Write([]byte(a.formatLineProtocol(prefix, tags, val)))
	return err
}

// NewEncoder creates a new encoder that writes to the given io.Writer.
func NewEncoder(w io.Writer) *Encoder {
	a := &Encoder{
		host:   fqdn.Get(),
		Writer: w,
	}
	return a
}
