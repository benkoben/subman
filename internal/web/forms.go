package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxFormBytes = 1 << 20

func parseForm(w http.ResponseWriter, r *http.Request) (url.Values, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("the submitted form could not be read")
	}
	return r.PostForm, nil
}

func requiredValue(form url.Values, name, label string) (string, error) {
	value := strings.TrimSpace(form.Get(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return value, nil
}

func positiveInt64(form url.Values, name, label string) (int64, error) {
	value, err := requiredValue(form, name, label)
	if err != nil {
		return 0, err
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("%s must be a positive whole number", label)
	}
	return number, nil
}

func positiveInt(form url.Values, name, label string) (int, error) {
	number, err := positiveInt64(form, name, label)
	if err != nil {
		return 0, err
	}
	converted := int(number)
	if int64(converted) != number {
		return 0, fmt.Errorf("%s is too large", label)
	}
	return converted, nil
}

func utcDateTime(form url.Values, name, label string) (time.Time, error) {
	value, err := requiredValue(form, name, label)
	if err != nil {
		return time.Time{}, err
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04"} {
		if parsed, parseErr := time.ParseInLocation(layout, value, time.UTC); parseErr == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("%s must be a valid UTC date and time", label)
}
