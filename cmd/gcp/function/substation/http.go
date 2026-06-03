package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/brexhq/substation/v2"
	"github.com/brexhq/substation/v2/message"

	"github.com/brexhq/substation/v2/internal/log"
)

const httpErrInternal = "internal server error\n"

// errInvalidJSON is returned when a transform produces invalid JSON and cannot be returned.
var errInvalidJSON = fmt.Errorf("transformed data is invalid JSON and cannot be returned")

type httpMetadata struct {
	Path    string              `json:"path"`
	Method  string              `json:"method"`
	Headers map[string][]string `json:"headers"`
}

func respondHTTPError(w http.ResponseWriter, err error) {
	log.WithField("error", err).Error("http handler")
	http.Error(w, httpErrInternal, http.StatusInternalServerError)
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	conf, err := getConfig(ctx)
	if err != nil {
		respondHTTPError(w, err)
		return
	}

	cfg := substation.Config{}
	if err := json.NewDecoder(conf).Decode(&cfg); err != nil {
		respondHTTPError(w, err)
		return
	}

	sub, err := substation.New(ctx, cfg)
	if err != nil {
		respondHTTPError(w, err)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondHTTPError(w, err)
		return
	}

	m := httpMetadata{
		Path:    r.URL.Path,
		Method:  r.Method,
		Headers: r.Header,
	}

	metadata, err := json.Marshal(m)
	if err != nil {
		respondHTTPError(w, err)
		return
	}

	msg := []*message.Message{
		message.New().SetData(body).SetMetadata(metadata),
		message.New().AsControl(),
	}

	res, err := sub.Transform(ctx, msg...)
	if err != nil {
		respondHTTPError(w, err)
		return
	}

	var output []json.RawMessage
	for _, msg := range res {
		if msg.IsControl() {
			continue
		}

		if !json.Valid(msg.Data()) {
			respondHTTPError(w, errInvalidJSON)
			return
		}

		var rm json.RawMessage
		if err := json.Unmarshal(msg.Data(), &rm); err != nil {
			respondHTTPError(w, err)
			return
		}

		output = append(output, rm)
	}

	resp, err := json.Marshal(output)
	if err != nil {
		respondHTTPError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp)
}
