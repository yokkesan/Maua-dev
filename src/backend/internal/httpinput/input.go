package httpinput

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strings"
)

func Decode(r *http.Request, value any) error {
	data, err := io.ReadAll(io.LimitReader(r.Body, 16385))
	if err != nil || len(data) > 16384 {
		return errors.New("invalid request")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("invalid request")
	}
	return nil
}
func Email(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || len(s) > 254 || strings.ContainsAny(s, "\r\n") {
		return "", errors.New("invalid email")
	}
	return s, nil
}
