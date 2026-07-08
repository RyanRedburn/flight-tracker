package query

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/go-playground/form/v4"
)

var queryDecoder = form.NewDecoder()

func init() {
	queryDecoder.SetTagName("query")
	queryDecoder.RegisterCustomTypeFunc(decodeDateOnly, time.Time{})
}

func decodeDateOnly(vals []string) (any, error) {
	if len(vals) == 0 || vals[0] == "" {
		return time.Time{}, nil
	}

	t, err := time.Parse("2006-01-02", strings.TrimSpace(vals[0]))
	if err != nil {
		return nil, err
	}

	return t, nil
}

// BindQuery decodes r.URL.Query() into dest using `query` struct tags.
func BindQuery(r *http.Request, dest any) error {
	values := cloneURLValues(r.URL.Query())
	expandCommaSeparated(values, "days_of_week")

	if err := queryDecoder.Decode(dest, values); err != nil {
		return formatBindError(err)
	}

	return nil
}

func cloneURLValues(in url.Values) url.Values {
	out := make(url.Values, len(in))
	for k, vs := range in {
		out[k] = append([]string(nil), vs...)
	}

	return out
}

// expandCommaSeparated rewrites single "a,b,c" values into repeated keys so
// go-playground/form can decode them into slices.
func expandCommaSeparated(values url.Values, keys ...string) {
	for _, key := range keys {
		raw := values[key]
		if len(raw) != 1 || !strings.Contains(raw[0], ",") {
			continue
		}

		parts := strings.Split(raw[0], ",")

		expanded := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			expanded = append(expanded, part)
		}

		values[key] = expanded
	}
}

func formatBindError(err error) error {
	de, ok := err.(form.DecodeErrors)
	if !ok {
		return err
	}

	messages := make([]string, 0, len(de))
	for field := range de {
		messages = append(messages, fieldName(field)+" is invalid")
	}

	return fmt.Errorf("%s", strings.Join(messages, "; "))
}

func fieldName(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		return path[i+1:]
	}

	return path
}

func normalizeQueryStrings(v any) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return
	}

	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)

		tag, _, _ := strings.Cut(field.Tag.Get("query"), ",")
		if tag == "" || tag == "-" {
			continue
		}

		fv := rv.Field(i)
		if !fv.CanSet() || fv.Kind() != reflect.String {
			continue
		}

		switch tag {
		case "origin", "dest", "carrier":
			fv.SetString(strings.ToUpper(strings.TrimSpace(fv.String())))
		default:
			fv.SetString(strings.TrimSpace(fv.String()))
		}
	}
}
