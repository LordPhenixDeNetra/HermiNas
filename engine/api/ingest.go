package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"herminas/kernel/errors"
)

// Inserter writes records straight to ClickHouse over its HTTP interface —
// used by the /api/v1/ingest/{dataset} handler, which is the Go-side
// ingestion path (cahier des charges §6.1) alongside the Rust agent's
// gRPC path (M1.2). Both converge on the same enrichment shape
// (received_at_unix_ms) so a dataset looks the same regardless of which
// path fed it.
type Inserter struct {
	baseURL string
	client  *http.Client
}

func NewInserter(baseURL string) *Inserter {
	return &Inserter{baseURL: baseURL, client: &http.Client{}}
}

func (in *Inserter) Insert(ctx context.Context, table string, records []map[string]any) error {
	if len(records) == 0 {
		return nil
	}

	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			return errors.Wrap(errors.CodeInternal, "encode record", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, in.baseURL, &body)
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "build insert request", err)
	}
	q := url.Values{"query": {fmt.Sprintf("INSERT INTO %s FORMAT JSONEachRow", quoteIdent(table))}}
	req.URL.RawQuery = q.Encode()

	resp, err := in.client.Do(req)
	if err != nil {
		return errors.Wrap(errors.CodeUnavailable, "clickhouse insert failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return errors.New(errors.CodeInternal, fmt.Sprintf("clickhouse returned %d: %s", resp.StatusCode, respBody))
	}
	return nil
}

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
