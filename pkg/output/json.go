package output

import (
	"encoding/json"
	"io"

	"github.com/network-doctor/network-doctor/pkg/diagnosis"
	"github.com/network-doctor/network-doctor/pkg/probe"
)

type JSONRenderer struct{}

type JSONOutput struct {
	Target     string                        `json:"target"`
	Reachable  bool                          `json:"reachable"`
	Probes     map[string]*probe.ProbeResult `json:"probes"`
	Diagnosis  string                        `json:"diagnosis"`
	Suggestion string                        `json:"suggestion,omitempty"`
	Warnings   []string                      `json:"warnings,omitempty"`
}

func (r *JSONRenderer) Render(w io.Writer, target string, results []*probe.ProbeResult, diag *diagnosis.Diagnosis, verbose bool) error {
	probes := make(map[string]*probe.ProbeResult)
	for _, result := range results {
		result.FinalizeStatus()
		probes[result.Name] = result
	}

	out := JSONOutput{
		Target:     target,
		Reachable:  diag.Reachable,
		Probes:     probes,
		Diagnosis:  diag.Summary,
		Suggestion: diag.Suggestion,
		Warnings:   diag.Warnings,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func (r *JSONRenderer) RenderBatch(w io.Writer, outputs []JSONOutput) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(outputs)
}
