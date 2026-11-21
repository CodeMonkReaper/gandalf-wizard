package schema

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

const maxDefinitionBytes int64 = 1 << 20 // 1 MiB

var (
	safeVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	safeNodePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
)

// LoadAndValidate reads, strictly parses, and validates a declarative workflow
// definition in read-only mode without invoking external providers.
func LoadAndValidate(defPath string, opts ValidateOptions) (ValidatedWorkflow, error) {
	raw, readErr := readDefinitionBytes(defPath)
	if readErr != nil {
		return ValidatedWorkflow{}, &ValidationErrors{Errors: []ValidationError{*readErr}}
	}

	sum := sha256.Sum256(raw)
	defSHA256 := hex.EncodeToString(sum[:])

	def, parseErr := decodeStrictDefinition(raw)
	if parseErr != nil {
		return ValidatedWorkflow{}, &ValidationErrors{Errors: []ValidationError{*parseErr}}
	}

	var errs []ValidationError
	addErr := func(code, field, msg string) {
		errs = append(errs, ValidationError{Code: code, Field: field, Message: msg})
	}

	if def.SchemaVersion != SchemaVersion {
		addErr("invalid_schema_version", "schema_version", fmt.Sprintf("unsupported schema_version %q (want %q)", def.SchemaVersion, SchemaVersion))
	}
	if !isSafeVersionID(def.ID) {
		addErr("invalid_identifier", "id", "workflow id must be 1..64 safe ASCII characters without '..'")
	}
	if !isSafeVersionID(def.Version) {
		addErr("invalid_identifier", "version", "workflow version must be 1..64 safe ASCII characters without '..'")
	}
	if def.ConcurrencyLimit < 0 || def.ConcurrencyLimit > MaxConcurrencyLimit {
		addErr("invalid_concurrency", "concurrency_limit", fmt.Sprintf("concurrency_limit must be between 1 and %d", MaxConcurrencyLimit))
	} else if def.ConcurrencyLimit == 0 {
		def.ConcurrencyLimit = DefaultConcurrencyLimit
	}

	deliverableSet := validateObjectiveHeader(&def.Objective, addErr)

	stepByID, uniqueSteps, stepOutputs, requiresMemory, requiresRouter := validateSteps(def.Steps, addErr)

	order, hasCycle := topologicalOrder(uniqueSteps, stepByID, addErr)

	var ancestors map[string]map[string]bool
	if !hasCycle {
		ancestors = computeAncestors(order, stepByID)
	}

	validateReferencesAndCorrections(&def, uniqueSteps, stepByID, stepOutputs, deliverableSet, ancestors, hasCycle, addErr)

	if opts.CheckProviders {
		if requiresMemory && !opts.MemoryReady {
			addErr("provider_unavailable", "providers.romero", "workflow contains memory_query steps but ROMERO is unavailable")
		}
		if requiresRouter && !opts.RouterReady {
			addErr("provider_unavailable", "providers.router", "workflow contains router_task steps but Router is unavailable")
		}
	}

	if len(errs) > 0 {
		return ValidatedWorkflow{}, &ValidationErrors{Errors: errs}
	}

	return ValidatedWorkflow{
		Definition:       def,
		DefinitionPath:   defPath,
		DefinitionSHA256: defSHA256,
		ExecutionOrder:   order,
		RequiresMemory:   requiresMemory,
		RequiresRouter:   requiresRouter,
	}, nil
}

func readDefinitionBytes(defPath string) ([]byte, *ValidationError) {
	if strings.TrimSpace(defPath) == "" {
		return nil, &ValidationError{Code: "io_error", Field: "definition", Message: "definition path is empty"}
	}
	info, err := os.Stat(defPath)
	if err != nil {
		return nil, &ValidationError{Code: "io_error", Field: "definition", Message: "cannot stat workflow definition"}
	}
	if info.IsDir() || !info.Mode().IsRegular() {
		return nil, &ValidationError{Code: "io_error", Field: "definition", Message: "workflow definition path is not a regular file"}
	}
	if info.Size() == 0 {
		return nil, &ValidationError{Code: "io_error", Field: "definition", Message: "workflow definition file is empty"}
	}
	if info.Size() > maxDefinitionBytes {
		return nil, &ValidationError{Code: "io_error", Field: "definition", Message: fmt.Sprintf("workflow definition exceeds %d bytes", maxDefinitionBytes)}
	}
	raw, err := os.ReadFile(defPath)
	if err != nil {
		return nil, &ValidationError{Code: "io_error", Field: "definition", Message: "cannot read workflow definition"}
	}
	if int64(len(raw)) > maxDefinitionBytes || len(bytes.TrimSpace(raw)) == 0 {
		return nil, &ValidationError{Code: "io_error", Field: "definition", Message: "workflow definition is empty or too large"}
	}
	return raw, nil
}

func decodeStrictDefinition(raw []byte) (WorkflowDefinition, *ValidationError) {
	var def WorkflowDefinition
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&def); err != nil {
		return WorkflowDefinition{}, &ValidationError{Code: "invalid_json", Field: "definition", Message: "invalid strict JSON definition"}
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return WorkflowDefinition{}, &ValidationError{Code: "invalid_json", Field: "definition", Message: "unexpected trailing JSON content"}
	}
	return def, nil
}

func isSafeVersionID(s string) bool {
	if len(s) == 0 || len(s) > MaxIdentifierLen || strings.Contains(s, "..") {
		return false
	}
	return safeVersionPattern.MatchString(s)
}

func isSafeNodeID(s string) bool {
	if len(s) == 0 || len(s) > MaxIdentifierLen || len(s) > MaxArtifactNameLen {
		return false
	}
	if isWindowsReservedDevice(s) {
		return false
	}
	return safeNodePattern.MatchString(s)
}

// isWindowsReservedDevice mirrors the store's confinement rule so preflight
// rejects artifact names the store would refuse after a provider already ran.
func isWindowsReservedDevice(name string) bool {
	upper := strings.ToUpper(strings.TrimRight(name, " "))
	switch upper {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(upper) == 4 && (strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT")) {
		return upper[3] >= '1' && upper[3] <= '9'
	}
	return false
}

type errAdder func(code, field, msg string)

func validateObjectiveHeader(obj *ObjectiveSpec, addErr errAdder) map[string]bool {
	if !isSafeVersionID(obj.ID) {
		addErr("invalid_identifier", "objective.id", "objective id must be 1..64 safe ASCII characters without '..'")
	}
	if !isSafeVersionID(obj.Version) {
		addErr("invalid_identifier", "objective.version", "objective version must be 1..64 safe ASCII characters without '..'")
	}
	if strings.TrimSpace(obj.Statement) == "" {
		addErr("invalid_objective", "objective.statement", "objective statement is required")
	}
	if len(obj.Deliverables) == 0 {
		addErr("invalid_objective", "objective.deliverables", "at least one objective deliverable is required")
	}
	if len(obj.SuccessCriteria) == 0 {
		addErr("invalid_objective", "objective.success_criteria", "at least one objective success criterion is required")
	}

	deliverableSet := make(map[string]bool, len(obj.Deliverables))
	for i, d := range obj.Deliverables {
		field := fmt.Sprintf("objective.deliverables[%d].name", i)
		if !isSafeNodeID(d.Name) {
			addErr("invalid_identifier", field, "deliverable name must be 1..64 safe ASCII characters")
			continue
		}
		if deliverableSet[d.Name] {
			addErr("invalid_objective", field, fmt.Sprintf("duplicate deliverable %q", d.Name))
			continue
		}
		deliverableSet[d.Name] = true
	}

	criterionSet := make(map[string]bool, len(obj.SuccessCriteria))
	for i, c := range obj.SuccessCriteria {
		field := fmt.Sprintf("objective.success_criteria[%d]", i)
		if !isSafeVersionID(c.ID) {
			addErr("invalid_identifier", field+".id", "criterion id must be 1..64 safe ASCII characters without '..'")
		} else if criterionSet[c.ID] {
			addErr("invalid_objective", field+".id", fmt.Sprintf("duplicate criterion %q", c.ID))
		} else {
			criterionSet[c.ID] = true
		}
		if c.Informative {
			addErr("non_informative_required", field+".informative", "objective success criteria cannot be marked informative")
		}
		if len(c.Rules) == 0 {
			addErr("invalid_rule", field+".rules", "criterion requires at least one deterministic rule")
		} else {
			validateRules(field+".rules", c.Rules, false, addErr)
		}
	}

	return deliverableSet
}

func validateSteps(steps []StepSpec, addErr errAdder) (map[string]StepSpec, []StepSpec, map[string]map[string]bool, bool, bool) {
	if len(steps) == 0 {
		addErr("invalid_step", "steps", "workflow must declare at least one step")
		return nil, nil, nil, false, false
	}

	stepByID := make(map[string]StepSpec, len(steps))
	uniqueSteps := make([]StepSpec, 0, len(steps))
	stepOutputs := make(map[string]map[string]bool, len(steps))
	var requiresMemory, requiresRouter bool

	for i, s := range steps {
		prefix := fmt.Sprintf("steps[%d]", i)
		if !isSafeNodeID(s.ID) {
			addErr("invalid_identifier", prefix+".id", "step id must be 1..64 safe ASCII characters")
			continue
		}
		if _, exists := stepByID[s.ID]; exists {
			addErr("duplicate_step", prefix+".id", fmt.Sprintf("duplicate step id %q", s.ID))
			continue
		}
		stepByID[s.ID] = s
		uniqueSteps = append(uniqueSteps, s)

		outSet := make(map[string]bool, len(s.Outputs))
		for j, outName := range s.Outputs {
			outField := fmt.Sprintf("%s.outputs[%d]", prefix, j)
			if !isSafeNodeID(outName) {
				addErr("invalid_identifier", outField, "output artifact name must be 1..64 safe ASCII characters")
				continue
			}
			if outSet[outName] {
				addErr("invalid_step", outField, fmt.Sprintf("duplicate output %q in step %q", outName, s.ID))
				continue
			}
			outSet[outName] = true
		}
		stepOutputs[s.ID] = outSet

		switch s.Kind {
		case StepKindMemoryQuery:
			requiresMemory = true
			if s.Memory == nil || strings.TrimSpace(s.Memory.Query) == "" {
				addErr("invalid_step", prefix+".memory", "memory_query requires non-empty memory.query")
			}
			if s.Router != nil || s.Gate != nil || s.Approval != nil {
				addErr("invalid_step", prefix, "memory_query cannot include router, gate, or approval config")
			}
			if s.Correction != nil {
				addErr("invalid_correction", prefix+".correction", "correction is only allowed on router_task steps")
			}
		case StepKindRouterTask:
			requiresRouter = true
			if s.Router == nil || strings.TrimSpace(s.Router.Prompt) == "" {
				addErr("invalid_step", prefix+".router", "router_task requires non-empty router.prompt")
			} else {
				switch s.Router.Complexity {
				case "", "baja", "media", "alta":
				default:
					addErr("invalid_step", prefix+".router.complexity", fmt.Sprintf("unsupported complexity %q", s.Router.Complexity))
				}
				switch s.Router.Risk {
				case "", "bajo", "medio", "alto":
				default:
					addErr("invalid_step", prefix+".router.risk", fmt.Sprintf("unsupported risk %q", s.Router.Risk))
				}
			}
			if s.Memory != nil || s.Gate != nil || s.Approval != nil {
				addErr("invalid_step", prefix, "router_task cannot include memory, gate, or approval config")
			}
		case StepKindQualityGate:
			if s.Optional {
				addErr("invalid_step", prefix+".optional", "quality_gate cannot be optional; declare the gate informative instead")
			}
			if s.Gate == nil || len(s.Gate.Rules) == 0 {
				addErr("invalid_step", prefix+".gate", "quality_gate requires gate config with at least one rule")
			} else {
				validateRules(prefix+".gate.rules", s.Gate.Rules, s.Gate.Informative, addErr)
			}
			if s.Memory != nil || s.Router != nil || s.Approval != nil {
				addErr("invalid_step", prefix, "quality_gate cannot include memory, router, or approval config")
			}
			if s.Correction != nil {
				addErr("invalid_correction", prefix+".correction", "correction is only allowed on router_task steps")
			}
		case StepKindHumanApproval:
			if s.Approval == nil || strings.TrimSpace(s.Approval.Prompt) == "" {
				addErr("invalid_step", prefix+".approval", "human_approval requires non-empty approval.prompt")
			}
			if s.Memory != nil || s.Router != nil || s.Gate != nil {
				addErr("invalid_step", prefix, "human_approval cannot include memory, router, or gate config")
			}
			if s.Correction != nil {
				addErr("invalid_correction", prefix+".correction", "correction is only allowed on router_task steps")
			}
		default:
			addErr("invalid_step", prefix+".kind", fmt.Sprintf("unsupported step kind %q", s.Kind))
		}
	}

	return stepByID, uniqueSteps, stepOutputs, requiresMemory, requiresRouter
}

func validateRules(fieldPrefix string, rules []GateRuleSpec, gateInformative bool, addErr errAdder) {
	for i, r := range rules {
		rf := fmt.Sprintf("%s[%d]", fieldPrefix, i)
		switch r.Kind {
		case RuleNonEmpty, RuleJSONValid:
		case RuleNoSecrets:
			if gateInformative {
				addErr("non_informative_required", rf, "no_secrets rule cannot be marked informative")
			}
		case RuleMinLength, RuleMaxLength:
			if r.Limit < 1 {
				addErr("invalid_rule", rf+".limit", fmt.Sprintf("%s requires limit >= 1", r.Kind))
			}
		case RuleContains, RuleNotContains:
			if r.Value == "" {
				addErr("invalid_rule", rf+".value", fmt.Sprintf("%s requires non-empty value", r.Kind))
			}
		case RuleRegexMatch, RuleRegexForbid:
			if r.Value == "" {
				addErr("invalid_rule", rf+".value", fmt.Sprintf("%s requires non-empty regex pattern", r.Kind))
			} else if _, err := regexp.Compile(r.Value); err != nil {
				addErr("invalid_rule", rf+".value", fmt.Sprintf("invalid regex in %s", r.Kind))
			}
		case RuleJSONRequiredKeys:
			if len(r.Keys) == 0 {
				addErr("invalid_rule", rf+".keys", "json_required_keys requires at least one key")
			} else {
				for j, k := range r.Keys {
					if strings.TrimSpace(k) == "" {
						addErr("invalid_rule", fmt.Sprintf("%s.keys[%d]", rf, j), "json_required_keys entries cannot be empty")
					}
				}
			}
		default:
			addErr("invalid_rule", rf+".kind", fmt.Sprintf("unsupported rule kind %q", r.Kind))
		}
	}
}

func topologicalOrder(steps []StepSpec, stepByID map[string]StepSpec, addErr errAdder) ([]string, bool) {
	if len(steps) == 0 {
		return nil, false
	}

	indexOf := make(map[string]int, len(steps))
	for i, s := range steps {
		indexOf[s.ID] = i
	}

	indegree := make(map[string]int, len(steps))
	dependents := make(map[string][]string, len(steps))
	hasMissingOrSelf := false

	for _, s := range steps {
		seenDep := make(map[string]bool, len(s.DependsOn))
		for j, dep := range s.DependsOn {
			depField := fmt.Sprintf("step.%s.depends_on[%d]", s.ID, j)
			if dep == s.ID {
				addErr("cycle_detected", depField, fmt.Sprintf("step %q cannot depend on itself", s.ID))
				hasMissingOrSelf = true
				continue
			}
			if seenDep[dep] {
				addErr("invalid_step", depField, fmt.Sprintf("duplicate dependency %q in step %q", dep, s.ID))
				continue
			}
			seenDep[dep] = true
			if _, ok := stepByID[dep]; !ok {
				addErr("broken_dependency", depField, fmt.Sprintf("dependency %q does not exist", dep))
				hasMissingOrSelf = true
				continue
			}
			indegree[s.ID]++
			dependents[dep] = append(dependents[dep], s.ID)
		}
	}

	ready := make([]bool, len(steps))
	for i, s := range steps {
		if indegree[s.ID] == 0 {
			ready[i] = true
		}
	}

	order := make([]string, 0, len(steps))
	for len(order) < len(steps) {
		picked := -1
		for i := range steps {
			if ready[i] {
				picked = i
				break
			}
		}
		if picked == -1 {
			addErr("cycle_detected", "steps", "dependency cycle detected in workflow graph")
			return nil, true
		}
		ready[picked] = false
		id := steps[picked].ID
		order = append(order, id)
		for _, next := range dependents[id] {
			indegree[next]--
			if indegree[next] == 0 {
				ready[indexOf[next]] = true
			}
		}
	}

	return order, hasMissingOrSelf
}

func computeAncestors(order []string, stepByID map[string]StepSpec) map[string]map[string]bool {
	anc := make(map[string]map[string]bool, len(order))
	for _, id := range order {
		set := make(map[string]bool)
		for _, dep := range stepByID[id].DependsOn {
			set[dep] = true
			for a := range anc[dep] {
				set[a] = true
			}
		}
		anc[id] = set
	}
	return anc
}

func splitStepOutputRef(ref string) (string, string, bool) {
	stepID, outName, ok := strings.Cut(ref, ".")
	if !ok || stepID == "" || outName == "" || strings.Contains(outName, ".") {
		return "", "", false
	}
	if !isSafeNodeID(stepID) || !isSafeNodeID(outName) {
		return "", "", false
	}
	return stepID, outName, true
}

func validateReferencesAndCorrections(
	def *WorkflowDefinition,
	steps []StepSpec,
	stepByID map[string]StepSpec,
	stepOutputs map[string]map[string]bool,
	deliverableSet map[string]bool,
	ancestors map[string]map[string]bool,
	hasCycle bool,
	addErr errAdder,
) {
	for i, d := range def.Objective.Deliverables {
		field := fmt.Sprintf("objective.deliverables[%d].source", i)
		stepID, outName, ok := splitStepOutputRef(d.Source)
		if !ok || !stepOutputs[stepID][outName] {
			addErr("broken_reference", field, fmt.Sprintf("deliverable source %q does not resolve to a declared step output", d.Source))
		}
	}

	for i, c := range def.Objective.SuccessCriteria {
		if !deliverableSet[c.Deliverable] {
			addErr("broken_reference", fmt.Sprintf("objective.success_criteria[%d].deliverable", i), fmt.Sprintf("criterion %q references unknown deliverable %q", c.ID, c.Deliverable))
		}
	}

	claimedGates := make(map[string]string)
	for _, s := range steps {
		for k, ref := range s.Inputs {
			field := fmt.Sprintf("step.%s.inputs.%s", s.ID, k)
			if strings.TrimSpace(k) == "" {
				addErr("invalid_step", field, "input key cannot be empty")
				continue
			}
			srcStep, outName, ok := splitStepOutputRef(ref)
			if !ok || !stepOutputs[srcStep][outName] {
				addErr("broken_reference", field, fmt.Sprintf("input reference %q does not resolve to a declared step output", ref))
				continue
			}
			if !hasCycle && !ancestors[s.ID][srcStep] {
				addErr("broken_reference", field, fmt.Sprintf("input %q references step %q which is not an ancestor in depends_on", ref, srcStep))
			}
		}

		if s.Kind == StepKindQualityGate && s.Gate != nil {
			field := fmt.Sprintf("step.%s.gate.target", s.ID)
			srcStep, outName, ok := splitStepOutputRef(s.Gate.Target)
			if !ok || !stepOutputs[srcStep][outName] {
				addErr("broken_reference", field, fmt.Sprintf("gate target %q does not resolve to a declared step output", s.Gate.Target))
			} else if !hasCycle && !ancestors[s.ID][srcStep] {
				addErr("broken_reference", field, fmt.Sprintf("gate target %q must reference an ancestor step in depends_on", s.Gate.Target))
			}
		}

		if s.Kind == StepKindRouterTask && s.Correction != nil {
			cf := fmt.Sprintf("step.%s.correction", s.ID)
			if s.Correction.MaxIterations < 1 {
				addErr("invalid_correction", cf+".max_iterations", "correction max_iterations must be >= 1")
			}
			gateStep, exists := stepByID[s.Correction.GateStepID]
			if !exists || gateStep.Kind != StepKindQualityGate || gateStep.Gate == nil {
				addErr("invalid_correction", cf+".gate_step_id", fmt.Sprintf("correction gate_step_id %q must reference a quality_gate step", s.Correction.GateStepID))
				continue
			}
			if gateStep.Gate.Informative {
				addErr("invalid_correction", cf+".gate_step_id", fmt.Sprintf("correction gate %q must be blocking, not informative", gateStep.ID))
			}
			if !containsString(gateStep.DependsOn, s.ID) {
				addErr("invalid_correction", cf+".gate_step_id", fmt.Sprintf("correction gate %q must list %q in depends_on", gateStep.ID, s.ID))
			}
			if targetStep, _, ok := splitStepOutputRef(gateStep.Gate.Target); !ok || targetStep != s.ID {
				addErr("invalid_correction", cf+".gate_step_id", fmt.Sprintf("correction gate %q must target an output of step %q", gateStep.ID, s.ID))
			}
			if prev, dup := claimedGates[gateStep.ID]; dup {
				addErr("invalid_correction", cf+".gate_step_id", fmt.Sprintf("correction gate %q already bound to step %q", gateStep.ID, prev))
			} else {
				claimedGates[gateStep.ID] = s.ID
			}
		}
	}
}

func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
