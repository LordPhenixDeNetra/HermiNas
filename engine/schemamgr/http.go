package schemamgr

import (
	"encoding/json"
	"net/http"

	"herminas/kernel/errors"
)

// Handler exposes Store over HTTP. A thin net/http.ServeMux (Go 1.22+
// method+path patterns, no router dependency) so it's ready to mount into
// the real API server once M1.5 builds JWT auth and the shared router —
// mounting it into a server that doesn't exist yet would be premature.
type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// Routes returns the /api/v1/datasets* handlers described in the cahier
// des charges §6.1.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/datasets", h.create)
	mux.HandleFunc("GET /api/v1/datasets", h.list)
	mux.HandleFunc("GET /api/v1/datasets/{name}", h.get)
	mux.HandleFunc("GET /api/v1/datasets/{name}/versions", h.versions)
	mux.HandleFunc("POST /api/v1/datasets/{name}/columns", h.addColumns)
	return mux
}

type createRequest struct {
	Name              string   `json:"name"`
	Columns           []Column `json:"columns"`
	OrderBy           []string `json:"order_by"`
	PartitionByColumn string   `json:"partition_by_column"`
	TTLDays           int      `json:"ttl_days"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	created, err := h.store.Create(Dataset{
		Name:              req.Name,
		Columns:           req.Columns,
		OrderBy:           req.OrderBy,
		PartitionByColumn: req.PartitionByColumn,
		TTLDays:           req.TTLDays,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) list(w http.ResponseWriter, _ *http.Request) {
	list, err := h.store.List()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	d, err := h.store.Get(r.PathValue("name"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) versions(w http.ResponseWriter, r *http.Request) {
	versions, err := h.store.Versions(r.PathValue("name"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

type addColumnsRequest struct {
	Columns []Column `json:"columns"`
}

func (h *Handler) addColumns(w http.ResponseWriter, r *http.Request) {
	var req addColumnsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	d, err := h.store.AddColumns(r.PathValue("name"), req.Columns)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func writeStoreError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.IsNotFound(err):
		status = http.StatusNotFound
	case errors.IsAlreadyExists(err):
		status = http.StatusConflict
	case errors.IsInvalidArgument(err):
		status = http.StatusBadRequest
	}
	writeError(w, status, err.Error())
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
