package gates

import (
	"strings"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
)

func syntheticSecrets() (mistral, openai, google, githubClassic, githubFine, pemHeader, pemBlock, dynamicSecret string) {
	mistral = "mstr" + "l_" + "SynthTestToken987654"
	openai = "sk-" + "proj-" + strings.Repeat("aB3d", 7)
	google = "AI" + "za" + strings.Repeat("Xy9Z", 9)
	githubClassic = "gh" + "p_" + strings.Repeat("kLmN", 8)
	githubFine = "github_" + "pat_" + strings.Repeat("99Zz", 8)
	pemHeader = "-----BEGIN " + "PRIVATE KEY-----"
	pemBlock = pemHeader + "\nMIIBVAIBADANBgkqhkiG9w0BAQEFAASC\n-----END " + "PRIVATE KEY-----"
	dynamicSecret = "romero-live-" + "custom-token-42"
	return
}

func TestWorkflowGates(t *testing.T) {
	mistral, openai, google, githubClassic, githubFine, pemHeader, pemBlock, dynamicSecret := syntheticSecrets()

	t.Run("redaccion_y_deteccion_de_secretos", func(t *testing.T) {
		hybridSecret := "sk-" + "proj-" + strings.Repeat("aB3d", 6) + "+tail"
		secretList := []string{"", "   ", "romero-live-", dynamicSecret, hybridSecret}
		inputs := []struct {
			name      string
			raw       string
			secret    string
			wantExact string
		}{
			{name: "mistral", raw: "header " + mistral + " tail", secret: mistral},
			{name: "openai", raw: "bearer " + openai + " end", secret: openai},
			{name: "google", raw: "key=" + google, secret: google},
			{name: "github_classic", raw: "tok=" + githubClassic, secret: githubClassic},
			{name: "github_pat", raw: "tok=" + githubFine, secret: githubFine},
			{name: "pem_header", raw: "payload " + pemHeader, secret: pemHeader},
			{name: "pem_block", raw: "before\n" + pemBlock + "\nafter", secret: "MIIBVAIBADANBgkqhkiG9w0BAQEFAASC"},
			{name: "dynamic_longest_first", raw: "env=" + dynamicSecret, secret: dynamicSecret, wantExact: "env=" + RedactedPlaceholder},
			{name: "hybrid_dynamic_with_pattern_prefix", raw: "key=" + hybridSecret, secret: "+tail", wantExact: "key=" + RedactedPlaceholder},
		}

		for _, tc := range inputs {
			t.Run(tc.name, func(t *testing.T) {
				if !HasSecrets(tc.raw, secretList) {
					t.Fatalf("HasSecrets(%q) = false, want true", tc.name)
				}
				redacted := Redact(tc.raw, secretList)
				if strings.Contains(redacted, tc.secret) {
					t.Fatalf("Redact leaked secret material for %s: %q", tc.name, redacted)
				}
				if tc.wantExact != "" && redacted != tc.wantExact {
					t.Fatalf("Redact(%q) = %q, want exact %q", tc.name, redacted, tc.wantExact)
				}
				if !strings.Contains(redacted, RedactedPlaceholder) {
					t.Fatalf("Redact missing placeholder %q in %q", RedactedPlaceholder, redacted)
				}
				if HasSecrets(redacted, secretList) {
					t.Fatalf("redacted string still triggers HasSecrets: %q", redacted)
				}
			})
		}

		clean := "resumen tecnico sin credenciales ni llaves"
		if HasSecrets(clean, secretList) {
			t.Fatalf("HasSecrets(clean) = true, want false")
		}
		if got := Redact(clean, secretList); got != clean {
			t.Fatalf("Redact(clean) = %q, want %q", got, clean)
		}
	})

	t.Run("reglas_deterministas_del_catalogo_cerrado", func(t *testing.T) {
		cases := []struct {
			name       string
			spec       schema.GateSpec
			raw        string
			wantStatus string
			wantRetry  bool
			wantInfo   bool
			wantInViol string
		}{
			{
				name:       "non_empty_pass",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleNonEmpty}}},
				raw:        "contenido valido",
				wantStatus: GateStatusPass,
			},
			{
				name:       "non_empty_whitespace_fail",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleNonEmpty}}},
				raw:        "  \n\t  ",
				wantStatus: GateStatusFail,
				wantRetry:  true,
			},
			{
				name:       "min_length_utf8_runes_pass",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleMinLength, Limit: 5}}},
				raw:        "áéíóú",
				wantStatus: GateStatusPass,
			},
			{
				name:       "min_length_utf8_runes_fail",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleMinLength, Limit: 6}}},
				raw:        "áéíóú",
				wantStatus: GateStatusFail,
				wantRetry:  true,
			},
			{
				name:       "max_length_pass",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleMaxLength, Limit: 5}}},
				raw:        "áéíóú",
				wantStatus: GateStatusPass,
			},
			{
				name:       "max_length_fail",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleMaxLength, Limit: 4}}},
				raw:        "áéíóú",
				wantStatus: GateStatusFail,
				wantRetry:  true,
			},
			{
				name:       "contains_and_not_contains_pass",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleContains, Value: "Veredicto:"}, {Kind: schema.RuleNotContains, Value: "TODO"}}},
				raw:        "Veredicto: aprobado",
				wantStatus: GateStatusPass,
			},
			{
				name:       "not_contains_fail",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleNotContains, Value: "TODO"}}},
				raw:        "Veredicto: pendiente TODO",
				wantStatus: GateStatusFail,
				wantRetry:  true,
			},
			{
				name:       "regex_match_and_forbid_pass",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleRegexMatch, Value: `^RISK:\s+(LOW|MEDIUM|HIGH)$`}, {Kind: schema.RuleRegexForbid, Value: `(?i)forbidden`}}},
				raw:        "RISK: LOW",
				wantStatus: GateStatusPass,
			},
			{
				name:       "regex_match_fail",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleRegexMatch, Value: `^RISK:\s+LOW$`}}},
				raw:        "RISK: HIGH",
				wantStatus: GateStatusFail,
				wantRetry:  true,
			},
			{
				name:       "regex_invalid_fails_closed",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleRegexMatch, Value: `[unclosed`}}},
				raw:        "anything",
				wantStatus: GateStatusFail,
				wantRetry:  true,
			},
			{
				name:       "json_valid_pass",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleJSONValid}}},
				raw:        `{"ok":true,"count":2}`,
				wantStatus: GateStatusPass,
			},
			{
				name:       "json_valid_trailing_garbage_fail",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleJSONValid}}},
				raw:        `{"ok":true} garbage`,
				wantStatus: GateStatusFail,
				wantRetry:  true,
			},
			{
				name:       "json_required_keys_pass",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleJSONRequiredKeys, Keys: []string{"summary", "risk"}}}},
				raw:        `{"summary":"listo","risk":"bajo"}`,
				wantStatus: GateStatusPass,
			},
			{
				name:       "json_required_keys_missing_key_fail",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleJSONRequiredKeys, Keys: []string{"summary", "risk"}}}},
				raw:        `{"summary":"listo"}`,
				wantStatus: GateStatusFail,
				wantRetry:  true,
				wantInViol: "risk",
			},
			{
				name:       "json_required_keys_null_fail",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleJSONRequiredKeys, Keys: []string{"summary"}}}},
				raw:        `null`,
				wantStatus: GateStatusFail,
				wantRetry:  true,
			},
			{
				name:       "json_required_keys_array_fail",
				spec:       schema.GateSpec{Target: "s.out", Rules: []schema.GateRuleSpec{{Kind: schema.RuleJSONRequiredKeys, Keys: []string{"summary"}}}},
				raw:        `[{"summary":"x"}]`,
				wantStatus: GateStatusFail,
				wantRetry:  true,
			},
			{
				name:       "empty_rules_fails_closed",
				spec:       schema.GateSpec{Target: "s.out"},
				raw:        "ok",
				wantStatus: GateStatusFail,
				wantRetry:  true,
			},
			{
				name:       "informative_mode_warning_without_retry",
				spec:       schema.GateSpec{Target: "s.out", Informative: true, Rules: []schema.GateRuleSpec{{Kind: schema.RuleMinLength, Limit: 20}}},
				raw:        "corto",
				wantStatus: GateStatusFail,
				wantRetry:  false,
				wantInfo:   true,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got := EvaluateGate(tc.spec, tc.raw, nil)
				if got.Status != tc.wantStatus {
					t.Fatalf("Status = %q, want %q (violations=%v, warnings=%v)", got.Status, tc.wantStatus, got.Violations, got.Warnings)
				}
				if got.Retryable != tc.wantRetry {
					t.Fatalf("Retryable = %v, want %v", got.Retryable, tc.wantRetry)
				}
				if got.Informative != tc.wantInfo {
					t.Fatalf("Informative = %v, want %v", got.Informative, tc.wantInfo)
				}
				if tc.wantStatus != GateStatusPass && len(got.Violations) == 0 {
					t.Fatalf("expected non-empty violations for status %q", got.Status)
				}
				if tc.wantInfo && len(got.Warnings) == 0 {
					t.Fatalf("expected warnings in informative failure")
				}
				if tc.wantInViol != "" && !strings.Contains(strings.Join(got.Violations, " | "), tc.wantInViol) {
					t.Fatalf("violations %v do not mention %q", got.Violations, tc.wantInViol)
				}
			})
		}
	})

	t.Run("no_secrets_evalua_primero_y_bloquea_sin_revelar_ni_permitir_informativo", func(t *testing.T) {
		spec := schema.GateSpec{
			Target:      "s.out",
			Informative: true,
			Rules: []schema.GateRuleSpec{
				{Kind: schema.RuleContains, Value: "esperado"},
				{Kind: schema.RuleNoSecrets},
			},
		}
		raw := "salida rota con " + mistral + " y " + dynamicSecret
		got := EvaluateGate(spec, raw, []string{dynamicSecret})
		if got.Status != GateStatusBlocked {
			t.Fatalf("Status = %q, want %q", got.Status, GateStatusBlocked)
		}
		if got.Retryable {
			t.Fatalf("no_secrets must not be retryable")
		}
		if got.Informative {
			t.Fatalf("no_secrets must never degrade to informative")
		}
		if len(got.Violations) == 0 {
			t.Fatalf("expected blocked violation message")
		}
		joined := strings.Join(append(append([]string(nil), got.Violations...), got.Warnings...), " ")
		if strings.Contains(joined, mistral) || strings.Contains(joined, dynamicSecret) {
			t.Fatalf("gate result leaked secret material: %q", joined)
		}
	})

	t.Run("evaluacion_de_objetivo_q2", func(t *testing.T) {
		baseObj := schema.ObjectiveSpec{
			ID:        "OBJ-PROP-001",
			Version:   "1.0.0",
			Statement: "Emitir analisis consolidado con veredicto JSON",
			Deliverables: []schema.DeliverableRef{
				{Name: "analisis_final", Source: "sintesis.report"},
			},
			SuccessCriteria: []schema.CriterionSpec{
				{
					ID:          "CRIT-01",
					Deliverable: "analisis_final",
					Rules: []schema.GateRuleSpec{
						{Kind: schema.RuleJSONValid},
						{Kind: schema.RuleJSONRequiredKeys, Keys: []string{"veredicto", "riesgos"}},
					},
				},
			},
		}

		goodJSON := `{"veredicto":"aprobar","riesgos":["lock"]}`

		t.Run("succeeded_conservando_warnings_informativos", func(t *testing.T) {
			verdict := EvaluateObjective(
				baseObj,
				map[string]StepOutcome{
					"contexto": {ID: "contexto", Status: "passed"},
					"revisor":  {ID: "revisor", Status: "passed", Warnings: []string{"advertencia menor"}},
					"sintesis": {ID: "sintesis", Status: "passed", Warnings: []string{"otra advertencia"}},
				},
				map[string]string{"analisis_final": goodJSON},
			)
			if verdict.Status != RunStatusSucceeded {
				t.Fatalf("Status = %q, want %q (failed=%v)", verdict.Status, RunStatusSucceeded, verdict.FailedCriteria)
			}
			if len(verdict.MetCriteria) != 1 || verdict.MetCriteria[0] != "CRIT-01" {
				t.Fatalf("MetCriteria = %v, want [CRIT-01]", verdict.MetCriteria)
			}
			if len(verdict.Warnings) != 2 {
				t.Fatalf("Warnings = %v, want 2 informative warnings", verdict.Warnings)
			}
		})

		t.Run("partial_cuando_solo_fallan_o_se_omiten_pasos_opcionales", func(t *testing.T) {
			verdict := EvaluateObjective(
				baseObj,
				map[string]StepOutcome{
					"sintesis": {ID: "sintesis", Status: "passed"},
					"extra":    {ID: "extra", Optional: true, Status: "skipped"},
				},
				map[string]string{"sintesis.report": goodJSON},
			)
			if verdict.Status != RunStatusPartial {
				t.Fatalf("Status = %q, want %q", verdict.Status, RunStatusPartial)
			}
		})

		t.Run("failed_si_paso_obligatorio_falla_aunque_opcional_pase_y_criterios_cumplan", func(t *testing.T) {
			verdict := EvaluateObjective(
				baseObj,
				map[string]StepOutcome{
					"contexto": {ID: "contexto", Optional: false, Status: "failed"},
					"extra":    {ID: "extra", Optional: true, Status: "failed"},
					"sintesis": {ID: "sintesis", Optional: false, Status: "passed"},
				},
				map[string]string{"analisis_final": goodJSON},
			)
			if verdict.Status != RunStatusFailed {
				t.Fatalf("Status = %q, want %q", verdict.Status, RunStatusFailed)
			}
		})

		t.Run("failed_por_criterio_no_cumplido_nunca_informativo", func(t *testing.T) {
			obj := baseObj
			obj.SuccessCriteria = []schema.CriterionSpec{
				{
					ID:          "CRIT-STRICT",
					Deliverable: "analisis_final",
					Informative: true, // defensivo: criterios del objetivo nunca son informativos
					Rules: []schema.GateRuleSpec{
						{Kind: schema.RuleJSONRequiredKeys, Keys: []string{"veredicto", "plan_rollback"}},
					},
				},
			}
			verdict := EvaluateObjective(
				obj,
				map[string]StepOutcome{
					"sintesis": {ID: "sintesis", Status: "passed"},
				},
				map[string]string{"analisis_final": goodJSON},
			)
			if verdict.Status != RunStatusFailed {
				t.Fatalf("Status = %q, want %q", verdict.Status, RunStatusFailed)
			}
			if len(verdict.FailedCriteria) != 1 || verdict.FailedCriteria[0] != "CRIT-STRICT" {
				t.Fatalf("FailedCriteria = %v, want [CRIT-STRICT]", verdict.FailedCriteria)
			}
		})

		t.Run("failed_por_entregable_faltante", func(t *testing.T) {
			verdict := EvaluateObjective(
				baseObj,
				map[string]StepOutcome{"sintesis": {ID: "sintesis", Status: "passed"}},
				map[string]string{},
			)
			if verdict.Status != RunStatusFailed {
				t.Fatalf("Status = %q, want %q", verdict.Status, RunStatusFailed)
			}
		})

		t.Run("blocked_por_paso_con_gate_blocked_y_redacta_warnings", func(t *testing.T) {
			verdict := EvaluateObjectiveWithSecrets(
				baseObj,
				map[string]StepOutcome{
					"sintesis": {
						ID:         "sintesis",
						Status:     "passed",
						GateStatus: GateStatusBlocked,
						Warnings:   []string{"nota con " + dynamicSecret},
					},
				},
				map[string]string{"analisis_final": goodJSON},
				[]string{dynamicSecret},
			)
			if verdict.Status != RunStatusBlocked {
				t.Fatalf("Status = %q, want %q", verdict.Status, RunStatusBlocked)
			}
			if len(verdict.Warnings) != 1 || verdict.Warnings[0] != "nota con "+RedactedPlaceholder {
				t.Fatalf("Warnings = %v, want redacted warning", verdict.Warnings)
			}
		})

		t.Run("blocked_por_secreto_patron_fijo_en_entregable", func(t *testing.T) {
			verdict := EvaluateObjectiveWithSecrets(
				baseObj,
				map[string]StepOutcome{
					"sintesis": {ID: "sintesis", Status: "passed"},
				},
				map[string]string{"analisis_final": `{"veredicto":"` + mistral + `","riesgos":[]}`},
				nil,
			)
			if verdict.Status != RunStatusBlocked {
				t.Fatalf("Status = %q, want %q", verdict.Status, RunStatusBlocked)
			}
		})

		t.Run("blocked_por_secretValues_dinamico_en_entregable", func(t *testing.T) {
			verdict := EvaluateObjectiveWithSecrets(
				baseObj,
				map[string]StepOutcome{
					"sintesis": {ID: "sintesis", Status: "passed"},
				},
				map[string]string{"analisis_final": `{"veredicto":"` + dynamicSecret + `","riesgos":[]}`},
				[]string{dynamicSecret},
			)
			if verdict.Status != RunStatusBlocked {
				t.Fatalf("Status = %q, want %q", verdict.Status, RunStatusBlocked)
			}
		})
	})
}
