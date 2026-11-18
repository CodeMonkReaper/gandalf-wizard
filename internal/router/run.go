package router

import (
	"context"
	"errors"
	"strings"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
)

// ResultStatus classifies the outcome of a run.
type ResultStatus string

const (
	StatusOK         ResultStatus = "ok"
	StatusNoEligible ResultStatus = "no-eligible"
	StatusExhausted  ResultStatus = "exhausted"
	StatusCanceled   ResultStatus = "canceled"
)

// AttemptOutcome classifies one attempt.
type AttemptOutcome string

const (
	AttemptOK       AttemptOutcome = "ok"
	AttemptFailed   AttemptOutcome = "failed"
	AttemptCanceled AttemptOutcome = "canceled"
)

// Attempt records one execution attempt with a sanitized reason.
type Attempt struct {
	Profile config.RouterProfile
	Effort  string
	Outcome AttemptOutcome
	Reason  string
}

// Result is the normalized outcome of a run.
type Result struct {
	Status      ResultStatus
	Profile     config.RouterProfile
	Effort      string
	Attempts    []Attempt
	Explanation string
	Output      string
}

// Run executes the decision's ordered choices sequentially with fallback.
// It is the only component that calls adapters: selection stays pure and
// the CLI only reports. Reasons and output are sanitized; raw executor
// errors and credential values never leave this function.
func Run(ctx context.Context, policy config.RouterPolicy, decision Decision, task string, reg *Registry, getenv func(string) string) Result {
	if ctx.Err() != nil {
		return Result{Status: StatusCanceled}
	}
	if len(decision.Ordered) == 0 {
		return Result{Status: StatusNoEligible}
	}

	secrets := collectSecrets(policy, getenv)
	redact := func(s string) string { return redactString(s, secrets) }

	res := Result{}
	seen := map[string]bool{}

	for _, choice := range decision.Ordered {
		if ctx.Err() != nil {
			res.Status = StatusCanceled
			break
		}
		if seen[choice.Profile.ID] {
			continue
		}
		seen[choice.Profile.ID] = true

		attempt := Attempt{Profile: choice.Profile, Effort: choice.Effort}

		adapter, executor, reason := resolveAttempt(policy, reg, choice)
		if reason != "" {
			attempt.Outcome = AttemptFailed
			attempt.Reason = redact(reason)
			res.Attempts = append(res.Attempts, attempt)
			if decision.Manual {
				break
			}
			continue
		}

		attemptCtx := ctx
		cancel := func() {}
		if policy.AttemptTimeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, policy.AttemptTimeout)
		}
		output, err := adapter.Execute(attemptCtx, Execution{
			Executor: executor,
			Profile:  choice.Profile,
			Effort:   choice.Profile.Efforts[choice.Effort],
			Task:     task,
		})
		cancel()

		if ctx.Err() != nil {
			attempt.Outcome = AttemptCanceled
			attempt.Reason = "cancelado"
			res.Attempts = append(res.Attempts, attempt)
			res.Status = StatusCanceled
			break
		}

		if err == nil {
			attempt.Outcome = AttemptOK
			res.Attempts = append(res.Attempts, attempt)
			res.Status = StatusOK
			res.Profile = choice.Profile
			res.Effort = choice.Effort
			res.Output = redact(output)
			res.Explanation = redact(choice.Explanation)
			if len(res.Attempts) > 1 {
				res.Explanation += "; se usó el fallback tras el fallo de los perfiles anteriores"
			}
			break
		}

		attempt.Outcome = AttemptFailed
		attempt.Reason = redact(failureReason(attemptCtx, err))
		res.Attempts = append(res.Attempts, attempt)
		if decision.Manual {
			break
		}
	}

	if res.Status == "" {
		res.Status = StatusExhausted
	}
	return res
}

// resolveAttempt revalidates one choice against the effective policy and
// resolves its adapter. A non-empty reason means the attempt cannot run.
func resolveAttempt(policy config.RouterPolicy, reg *Registry, choice Choice) (Adapter, config.RouterExecutor, string) {
	executor, ok := findExecutor(policy, choice.Profile.ExecutorID)
	if !ok {
		return nil, config.RouterExecutor{}, "ejecutor no declarado en la política"
	}
	if !executor.Authorized {
		return nil, executor, "ejecutor no autorizado"
	}
	if reg == nil {
		return nil, executor, "adaptador no registrado"
	}
	adapter, ok := reg.AdapterFor(choice.Profile.ExecutorID)
	if !ok {
		return nil, executor, "adaptador no registrado"
	}
	return adapter, executor, ""
}

// findExecutor locates an executor by ID in the effective policy.
func findExecutor(policy config.RouterPolicy, id string) (config.RouterExecutor, bool) {
	for _, e := range policy.Executors {
		if e.ID == id {
			return e, true
		}
	}
	return config.RouterExecutor{}, false
}

// failureReason maps an execution error to a sanitized, stable reason.
// The attempt deadline wins over the adapter's own classification; raw
// errors collapse to a generic message.
func failureReason(attemptCtx context.Context, err error) string {
	if attemptCtx.Err() == context.DeadlineExceeded {
		return "tiempo límite"
	}
	var sf *SafeFailure
	if errors.As(err, &sf) {
		if sf.Kind == FailureTimeout {
			return "tiempo límite"
		}
		if sf.Reason != "" {
			return sf.Reason
		}
		switch sf.Kind {
		case FailureUnavailable:
			return "el ejecutor no está disponible"
		case FailureInvalidOutput:
			return "salida inválida del ejecutor"
		}
	}
	return "el ejecutor terminó con error"
}

// collectSecrets reads the declared credential values of the policy's
// executors so Run can redact them from output and reasons.
func collectSecrets(policy config.RouterPolicy, getenv func(string) string) []string {
	if getenv == nil {
		return nil
	}
	var secrets []string
	for _, e := range policy.Executors {
		if e.CredentialEnv == "" {
			continue
		}
		if v := getenv(e.CredentialEnv); v != "" {
			secrets = append(secrets, v)
		}
	}
	return secrets
}

// redactString replaces every occurrence of a known secret.
func redactString(s string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "[redactado]")
		}
	}
	return s
}
