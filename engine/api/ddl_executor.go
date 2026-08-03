package api

import (
	"context"
	"io"
	"net/http"
	"strings"

	"herminas/kernel/errors"
)

// ClickHouseDDLExecutor implements schemamgr.DDLExecutor against a real
// ClickHouse instance: this is what makes `POST /api/v1/datasets` and
// `POST /api/v1/datasets/{name}/columns` actually create/alter a table,
// not just persist metadata.
type ClickHouseDDLExecutor struct {
	baseURL string
	client  *http.Client
}

func NewClickHouseDDLExecutor(baseURL string) *ClickHouseDDLExecutor {
	return &ClickHouseDDLExecutor{baseURL: baseURL, client: &http.Client{}}
}

func (e *ClickHouseDDLExecutor) Exec(ctx context.Context, ddl string) error {
	// DDL goes in the request body with no `query` URL param — simpler
	// than URL-encoding a multi-line CREATE TABLE/ALTER TABLE statement,
	// and ClickHouse's HTTP interface accepts SQL either way.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL, strings.NewReader(ddl))
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "build DDL request", err)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return errors.Wrap(errors.CodeUnavailable, "clickhouse DDL request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return errors.New(errors.CodeInternal, "clickhouse rejected DDL: "+strings.TrimSpace(string(body)))
	}
	return nil
}
