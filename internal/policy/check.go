// Package policy rejects privileged workload specs and records network assumptions.
// Kyverno is the selected admission policy. This package is the check that runs in CI.
package policy

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Admission is the controller this repository selected. It is not installed here.
const Admission = "Kyverno ClusterPolicy deploy/policy/kyverno-privileged.yaml. Kyverno was not installed in this session, so this checker is the rejection that ran."

// CNI records the kind network limit. No packet was dropped in this session.
const CNI = "Kind's current network plugin does not enforce NetworkPolicy. Default-deny manifests were not applied, and no denied network path was observed."

// Finding is one unsafe manifest rule.
type Finding struct {
	Kind   string
	Name   string
	Reason string
}

func (f Finding) String() string {
	return f.Kind + "/" + f.Name + ": " + f.Reason
}

// Intent is the network policy written for a later enforcing CNI.
type Intent struct {
	DefaultDeny    bool
	SameNamespace  bool
	DNS            bool
	ModelTCP443    bool
	ToolNetwork    bool
	Enforced       bool
	DeniedObserved bool
}

// CheckYAML returns unsafe findings for Pod, Deployment, and Job documents.
func CheckYAML(data []byte) ([]Finding, error) {
	var found []Finding
	for _, doc := range bytes.Split(data, []byte("\n---")) {
		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}
		var obj map[string]any
		if err := yaml.Unmarshal(doc, &obj); err != nil {
			return nil, err
		}
		if obj == nil {
			continue
		}
		found = append(found, checkObject(obj)...)
	}
	return found, nil
}

// CheckTree checks every .yaml and .yml file under root.
func CheckTree(root string) ([]Finding, error) {
	var found []Finding
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".yaml", ".yml":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		item, err := CheckYAML(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		found = append(found, item...)
		return nil
	})
	return found, err
}

// NetworkIntent reads default-deny and the gateway, model, and tool exceptions.
// Enforced and DeniedObserved stay false: this function does not apply the policy.
func NetworkIntent(data []byte) (Intent, error) {
	var intent Intent
	for _, doc := range bytes.Split(data, []byte("\n---")) {
		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}
		var obj map[string]any
		if err := yaml.Unmarshal(doc, &obj); err != nil {
			return Intent{}, err
		}
		if kindOf(obj) != "NetworkPolicy" {
			continue
		}
		spec, _ := obj["spec"].(map[string]any)
		if spec == nil {
			continue
		}
		types := stringList(spec["policyTypes"])
		_, ingress := spec["ingress"]
		_, egress := spec["egress"]
		if contains(types, "Ingress") && contains(types, "Egress") && !ingress && !egress {
			intent.DefaultDeny = true
		}
		if peerListHasPodSelector(spec["ingress"]) || peerListHasPodSelector(spec["egress"]) {
			intent.SameNamespace = true
		}
		if portsHave(spec["egress"], 53) {
			intent.DNS = true
		}
		if portsHave(spec["egress"], 443) {
			intent.ModelTCP443 = true
		}
	}
	return intent, nil
}

func checkObject(obj map[string]any) []Finding {
	spec := podSpec(obj)
	if spec == nil {
		return nil
	}
	kind := kindOf(obj)
	name := nameOf(obj)
	var found []Finding
	add := func(reason string) {
		found = append(found, Finding{Kind: kind, Name: name, Reason: reason})
	}
	if truthy(spec["hostNetwork"]) {
		add("hostNetwork")
	}
	if truthy(spec["hostPID"]) {
		add("hostPID")
	}
	if truthy(spec["hostIPC"]) {
		add("hostIPC")
	}
	for _, volume := range slice(spec["volumes"]) {
		if _, ok := volume["hostPath"]; ok {
			add("hostPath")
		}
	}
	podSafe, _ := spec["securityContext"].(map[string]any)
	for _, container := range append(append(slice(spec["containers"]), slice(spec["initContainers"])...), slice(spec["ephemeralContainers"])...) {
		security, _ := container["securityContext"].(map[string]any)
		if security == nil {
			security = map[string]any{}
		}
		if truthy(security["privileged"]) {
			add("privileged")
		}
		if escalation, ok := security["allowPrivilegeEscalation"].(bool); !ok || escalation {
			add("allowPrivilegeEscalation")
		}
		nonRoot := false
		if value, ok := security["runAsNonRoot"].(bool); ok && value {
			nonRoot = true
		}
		if value, ok := podSafe["runAsNonRoot"].(bool); ok && value {
			nonRoot = true
		}
		if !nonRoot {
			add("runAsNonRoot")
		}
		caps, _ := security["capabilities"].(map[string]any)
		dropped := false
		if caps != nil {
			for _, item := range stringList(caps["drop"]) {
				if item == "ALL" {
					dropped = true
				}
			}
			if len(sliceAny(caps["add"])) > 0 {
				add("added capability")
			}
		}
		if !dropped {
			add("capabilities drop ALL")
		}
		for _, port := range slice(container["ports"]) {
			if _, ok := port["hostPort"]; ok {
				add("hostPort")
			}
		}
	}
	return found
}

func podSpec(obj map[string]any) map[string]any {
	spec, _ := obj["spec"].(map[string]any)
	if spec == nil {
		return nil
	}
	switch kindOf(obj) {
	case "Pod":
		return spec
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job":
		template, _ := spec["template"].(map[string]any)
		if template == nil {
			return nil
		}
		nested, _ := template["spec"].(map[string]any)
		return nested
	case "CronJob":
		job, _ := spec["jobTemplate"].(map[string]any)
		if job == nil {
			return nil
		}
		jobSpec, _ := job["spec"].(map[string]any)
		if jobSpec == nil {
			return nil
		}
		template, _ := jobSpec["template"].(map[string]any)
		if template == nil {
			return nil
		}
		nested, _ := template["spec"].(map[string]any)
		return nested
	default:
		return nil
	}
}

func kindOf(obj map[string]any) string {
	kind, _ := obj["kind"].(string)
	return kind
}

func nameOf(obj map[string]any) string {
	meta, _ := obj["metadata"].(map[string]any)
	if meta == nil {
		return ""
	}
	name, _ := meta["name"].(string)
	return name
}

func truthy(value any) bool {
	flag, ok := value.(bool)
	return ok && flag
}

func slice(value any) []map[string]any {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		mapped, ok := item.(map[string]any)
		if ok {
			out = append(out, mapped)
		}
	}
	return out
}

func sliceAny(value any) []any {
	raw, _ := value.([]any)
	return raw
}

func stringList(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if ok {
			out = append(out, text)
		}
	}
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func peerListHasPodSelector(value any) bool {
	for _, rule := range slice(value) {
		peers := slice(rule["from"])
		peers = append(peers, slice(rule["to"])...)
		for _, peer := range peers {
			if _, ok := peer["podSelector"]; ok {
				return true
			}
		}
	}
	return false
}

func portsHave(value any, port int) bool {
	for _, rule := range slice(value) {
		for _, item := range slice(rule["ports"]) {
			if number(item["port"]) == port {
				return true
			}
		}
	}
	return false
}

func number(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case uint64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}
