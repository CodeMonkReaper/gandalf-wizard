// Package direct adapts direct HTTPS model APIs to the router port.
//
// One adapter serves every authorized API executor: the executor's
// protocol selects the request shape, so a provider compatible with a
// supported protocol is configured as another executor with no code
// changes. Requests carry no tools and no streaming, the transport never
// follows redirects, and only the executor's own credential is attached
// to its request.
package direct

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
)

// Protocol names supported by this adapter.
const (
	ProtocolOpenAI  = "openai"
	ProtocolMistral = "mistral"
	ProtocolGemini  = "gemini"
)

// Adapter implements router.Adapter for direct HTTPS APIs. The environment
// reader and the HTTP transport are injected so the automated tests use
// in-memory servers and never consume quota.
type Adapter struct {
	// Getenv reads one environment variable. Nil uses os.Getenv.
	Getenv func(string) string
	// Transport sends one request and returns its response without ever
	// following redirects. Nil uses http.DefaultTransport.
	Transport http.RoundTripper
}

var _ router.Adapter = (*Adapter)(nil)

// New returns the adapter with production defaults.
func New() *Adapter {
	return &Adapter{}
}

// supportedProtocol reports whether the protocol has a request shape here.
func supportedProtocol(protocol string) bool {
	return protocol == ProtocolOpenAI || protocol == ProtocolMistral || protocol == ProtocolGemini
}

// Probe reports the local, quota-free state of the executor: the API needs
// no binary, text-only is enforced by construction, and authentication
// means the declared credential variable is configured — never that it
// was validated remotely.
func (a *Adapter) Probe(ctx context.Context, ex config.RouterExecutor) router.Probe {
	_ = ctx
	if ex.Kind != config.RouterKindAPI {
		return router.Probe{Reason: "el adaptador no soporta este ejecutor"}
	}
	if !supportedProtocol(ex.Protocol) {
		return router.Probe{Reason: "protocolo no soportado"}
	}
	auth := router.AuthUnknown
	if ex.CredentialEnv != "" {
		if a.getenv(ex.CredentialEnv) != "" {
			auth = router.AuthReady
		} else {
			auth = router.AuthMissing
		}
	}
	return router.Probe{
		Installed: true,
		Auth:      auth,
		TextOnly:  router.TextOnlyEnforced,
		Reason:    "protocolo " + ex.Protocol + " sin herramientas ni streaming por diseño",
	}
}

// Discover reports that this stage offers no documented discovery for
// direct APIs; the catalog then uses the declared profiles.
func (a *Adapter) Discover(ctx context.Context, ex config.RouterExecutor) ([]router.Candidate, error) {
	_ = ctx
	if ex.Kind != config.RouterKindAPI {
		return nil, errors.New("direct: el adaptador no soporta este ejecutor")
	}
	return nil, router.ErrDiscoveryUnavailable
}

// Execute runs one text-only attempt against the executor's authorized
// HTTPS origin: one request, no redirects, no retries, and only the
// executor's own credential in its headers. Failures leave as sanitized
// SafeFailure values; bodies, URLs, and credentials never pass through.
func (a *Adapter) Execute(ctx context.Context, execution router.Execution) (string, error) {
	ex := execution.Executor
	if ex.Kind != config.RouterKindAPI {
		return "", &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "el adaptador no soporta este ejecutor"}
	}
	if !supportedProtocol(ex.Protocol) {
		return "", &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "protocolo no soportado"}
	}
	if ex.CredentialEnv == "" {
		return "", &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "credencial no declarada"}
	}
	credential := a.getenv(ex.CredentialEnv)
	if credential == "" {
		return "", &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "credencial no configurada"}
	}
	req, err := a.buildRequest(ctx, ex, execution, credential)
	if err != nil {
		return "", &router.SafeFailure{Kind: router.FailureUnavailable, Reason: "origen inválido"}
	}
	resp, err := a.transport().RoundTrip(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", mapCtxError(ctxErr)
		}
		return "", &router.SafeFailure{Kind: router.FailureExecutorError, Reason: "fallo de transporte"}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", &router.SafeFailure{Kind: router.FailureExecutorError, Reason: "fallo de transporte"}
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", mapCtxError(ctxErr)
	}
	if resp.StatusCode != http.StatusOK {
		return "", &router.SafeFailure{Kind: router.FailureExecutorError, Reason: "el proveedor respondió con error"}
	}
	return decodeResponse(ex.Protocol, body)
}

// buildRequest assembles the protocol's request: URL, headers, and the
// text-only payload with the declared effort translated where the
// protocol supports one.
func (a *Adapter) buildRequest(ctx context.Context, ex config.RouterExecutor, execution router.Execution, credential string) (*http.Request, error) {
	base := strings.TrimSuffix(ex.BaseURL, "/")
	var (
		requestURL string
		payload    any
	)
	switch ex.Protocol {
	case ProtocolOpenAI:
		requestURL = base + "/responses"
		p := openaiRequest{Model: execution.Profile.Model, Input: execution.Task}
		if execution.Effort != "" {
			p.Reasoning = &openaiReasoning{Effort: execution.Effort}
		}
		payload = p
	case ProtocolMistral:
		requestURL = base + "/chat/completions"
		payload = chatRequest{
			Model:    execution.Profile.Model,
			Messages: []chatMessage{{Role: "user", Content: execution.Task}},
		}
	case ProtocolGemini:
		requestURL = base + "/models/" + url.PathEscape(execution.Profile.Model) + ":generateContent"
		p := geminiRequest{
			Contents: []geminiContent{{
				Role:  "user",
				Parts: []geminiPart{{Text: execution.Task}},
			}},
		}
		if budget, err := strconv.Atoi(execution.Effort); err == nil && execution.Effort != "" {
			p.GenerationConfig = &geminiGenerationConfig{
				ThinkingConfig: &geminiThinkingConfig{ThinkingBudget: budget},
			}
		}
		payload = p
	default:
		return nil, errors.New("protocolo no soportado")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if req.URL.Scheme != "https" {
		return nil, errors.New("origen no https")
	}
	req.Header.Set("Content-Type", "application/json")
	switch ex.Protocol {
	case ProtocolGemini:
		req.Header.Set("x-goog-api-key", credential)
	default:
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	return req, nil
}

// decodeResponse extracts the final text of one protocol response.
// Reasoning, tool calls, and every other field are discarded; a response
// without useful text is an invalid output.
func decodeResponse(protocol string, body []byte) (string, error) {
	var text string
	switch protocol {
	case ProtocolOpenAI:
		var r openaiResponse
		if json.Unmarshal(body, &r) != nil {
			return "", invalidOutput()
		}
		var sb strings.Builder
		for _, item := range r.Output {
			if item.Type != "message" {
				continue
			}
			for _, part := range item.Content {
				if part.Type == "output_text" {
					sb.WriteString(part.Text)
				}
			}
		}
		text = sb.String()
	case ProtocolMistral:
		var r chatResponse
		if json.Unmarshal(body, &r) != nil {
			return "", invalidOutput()
		}
		for _, choice := range r.Choices {
			if strings.TrimSpace(choice.Message.Content) != "" {
				text = choice.Message.Content
			}
		}
	case ProtocolGemini:
		var r geminiResponse
		if json.Unmarshal(body, &r) != nil {
			return "", invalidOutput()
		}
		var sb strings.Builder
		for _, cand := range r.Candidates {
			for _, part := range cand.Content.Parts {
				if !part.Thought {
					sb.WriteString(part.Text)
				}
			}
		}
		text = sb.String()
	default:
		return "", invalidOutput()
	}
	if strings.TrimSpace(text) == "" {
		return "", invalidOutput()
	}
	return strings.TrimSpace(text), nil
}

func invalidOutput() error {
	return &router.SafeFailure{Kind: router.FailureInvalidOutput, Reason: "salida inválida del proveedor"}
}

// mapCtxError maps a done context to its sanitized failure. Cancellation
// passes through so the router reports it as cancellation; a deadline is
// a timeout.
func mapCtxError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	return &router.SafeFailure{Kind: router.FailureTimeout, Reason: "tiempo límite"}
}

func (a *Adapter) getenv(name string) string {
	if a.Getenv == nil {
		return os.Getenv(name)
	}
	return a.Getenv(name)
}

func (a *Adapter) transport() http.RoundTripper {
	if a.Transport == nil {
		return http.DefaultTransport
	}
	return a.Transport
}

// OpenAI Responses protocol shapes.

type openaiRequest struct {
	Model     string           `json:"model"`
	Input     string           `json:"input"`
	Reasoning *openaiReasoning `json:"reasoning,omitempty"`
}

type openaiReasoning struct {
	Effort string `json:"effort"`
}

type openaiResponse struct {
	Output []openaiOutputItem `json:"output"`
}

type openaiOutputItem struct {
	Type    string          `json:"type"`
	Content []openaiContent `json:"content"`
}

type openaiContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Chat Completions protocol shapes (Mistral and compatible providers).

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message chatResponseMessage `json:"message"`
}

type chatResponseMessage struct {
	Content string `json:"content"`
}

// Gemini GenerateContent protocol shapes.

type geminiRequest struct {
	Contents         []geminiContent         `json:"contents"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	ThinkingConfig *geminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

type geminiThinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiCandidateContent `json:"content"`
}

type geminiCandidateContent struct {
	Parts []geminiResponsePart `json:"parts"`
}

type geminiResponsePart struct {
	Text    string `json:"text"`
	Thought bool   `json:"thought,omitempty"`
}
