package romero

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	maxResponseBodyBytes = 4 << 20 // 4 MiB
	zeroUUID             = "00000000-0000-0000-0000-000000000000"
)

// RomeroPublicClientAdapter implements RomeroReadPort over ROMERO's public REST contract.
// Uses only Go stdlib net/http; zero external ROMERO dependency.
type RomeroPublicClientAdapter struct {
	baseURL           string
	httpClient        *http.Client
	authTokenProvider func() string
}

var _ RomeroReadPort = (*RomeroPublicClientAdapter)(nil)

// NewRomeroPublicClientAdapter builds a REST adapter against ROMERO's public API.
func NewRomeroPublicClientAdapter(baseURL string, httpClient *http.Client, authTokenProvider func() string) *RomeroPublicClientAdapter {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &RomeroPublicClientAdapter{
		baseURL:           strings.TrimRight(baseURL, "/"),
		httpClient:        httpClient,
		authTokenProvider: authTokenProvider,
	}
}

// searchRequest mirrors pkg/romeroclient.SearchRequest (rev 3f46f9a).
type searchRequest struct {
	Query             string   `json:"query"`
	ProjectIDs        []string `json:"project_ids,omitempty"`
	Scopes            []string `json:"scopes,omitempty"`
	IncludeSuperseded bool     `json:"include_superseded,omitempty"`
}

// searchCandidate mirrors pkg/romeroclient.SearchCandidate (rev 3f46f9a).
type searchCandidate struct {
	ID              string `json:"id"`
	Summary         string `json:"summary"`
	Status          string `json:"status"`
	EpistemicStatus string `json:"epistemic_status"`
}

// searchResponse mirrors pkg/romeroclient.SearchResponse (rev 3f46f9a).
type searchResponse struct {
	Candidates   []searchCandidate `json:"candidates"`
	DegradedMode bool              `json:"degraded_mode"`
	TotalFound   int               `json:"total_found"`
}

// getMemoryResponse mirrors pkg/romeroclient.GetMemoryResponse (rev 3f46f9a).
type getMemoryResponse struct {
	Memory struct {
		ID              string  `json:"ID"`
		ProjectID       *string `json:"ProjectID,omitempty"`
		Status          string  `json:"Status"`
		EpistemicStatus string  `json:"EpistemicStatus"`
		PrimarySourceID string  `json:"PrimarySourceID"`
	} `json:"memory"`
	Relations []struct {
		FromMemoryID string `json:"FromMemoryID"`
		ToMemoryID   string `json:"ToMemoryID"`
		RelationType string `json:"RelationType"`
		IsActive     bool   `json:"IsActive"`
	} `json:"relations,omitempty"`
}

// Search implements RomeroReadPort via POST /api/v1/memories/search.
func (a *RomeroPublicClientAdapter) Search(ctx context.Context, scope ResolvedScope, query string) (SearchResult, error) {
	body := searchRequest{
		Query:             query,
		ProjectIDs:        []string{scope.ProjectID},
		Scopes:            []string{"project_shared"},
		IncludeSuperseded: true,
	}
	resp, err := a.do(ctx, http.MethodPost, "/api/v1/memories/search", body)
	if err != nil {
		return SearchResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return SearchResult{}, ErrAccessDenied
	}
	if resp.StatusCode != http.StatusOK {
		return SearchResult{}, fmt.Errorf("%w: search status %d", ErrServiceInterrupted, resp.StatusCode)
	}

	var sr searchResponse
	if err := decodeCapped(resp.Body, &sr); err != nil {
		return SearchResult{}, fmt.Errorf("%w: decode search: %v", ErrServiceInterrupted, err)
	}

	hits := make([]MemorySearchHit, 0, len(sr.Candidates))
	for _, c := range sr.Candidates {
		vigencia := VigenciaActive
		if c.Status == "superseded" {
			vigencia = VigenciaSuperseded
		}
		hits = append(hits, MemorySearchHit{
			MemoryID:       c.ID,
			ProjectID:      scope.ProjectID,
			Summary:        c.Summary,
			EpistemicState: c.EpistemicStatus,
			Status:         c.Status,
			VigenciaState:  vigencia,
		})
	}

	return SearchResult{
		Hits:              hits,
		Degraded:          sr.DegradedMode,
		DegradationNotice: degradedNotice(sr.DegradedMode),
	}, nil
}

// GetDetail implements RomeroReadPort via GET /api/v1/memories/{id}.
func (a *RomeroPublicClientAdapter) GetDetail(ctx context.Context, scope ResolvedScope, memoryID string) (MemoryDetail, error) {
	resp, err := a.do(ctx, http.MethodGet, "/api/v1/memories/"+url.PathEscape(memoryID), nil)
	if err != nil {
		return MemoryDetail{}, fmt.Errorf("%w: %v", ErrDetailInterrupted, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		return MemoryDetail{}, ErrAccessDenied
	}
	if resp.StatusCode != http.StatusOK {
		return MemoryDetail{}, fmt.Errorf("%w: getDetail status %d", ErrDetailInterrupted, resp.StatusCode)
	}

	var gr getMemoryResponse
	if err := decodeCapped(resp.Body, &gr); err != nil {
		return MemoryDetail{}, fmt.Errorf("%w: decode detail: %v", ErrDetailInterrupted, err)
	}

	sourceRef := gr.Memory.PrimarySourceID
	if sourceRef == "" || sourceRef == zeroUUID {
		sourceRef = ""
	}

	projectID := ""
	if gr.Memory.ProjectID != nil {
		projectID = *gr.Memory.ProjectID
	}

	supersededBy := ""
	if gr.Memory.Status == "superseded" {
		for _, rel := range gr.Relations {
			if rel.RelationType == "supersedes" && rel.IsActive && rel.FromMemoryID == gr.Memory.ID {
				supersededBy = rel.ToMemoryID
				break
			}
		}
	}

	return MemoryDetail{
		MemoryID:       gr.Memory.ID,
		ProjectID:      projectID,
		SourceRef:      sourceRef,
		Status:         gr.Memory.Status,
		EpistemicState: gr.Memory.EpistemicStatus,
		SupersededBy:   supersededBy,
	}, nil
}

func (a *RomeroPublicClientAdapter) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if token := a.authTokenProvider(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrServiceInterrupted, err)
	}
	return resp, nil
}

func decodeCapped(r io.Reader, v any) error {
	limited := io.LimitReader(r, maxResponseBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(data) > maxResponseBodyBytes {
		return errors.New("response body exceeds 4 MiB cap")
	}
	return json.Unmarshal(data, v)
}

func degradedNotice(degraded bool) string {
	if !degraded {
		return ""
	}
	return "Búsqueda en modo degradado"
}
