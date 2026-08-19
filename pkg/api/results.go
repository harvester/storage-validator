package api

type Report struct {
	// Warnings highlights caveats about the run itself (as opposed to a
	// per-check failure) that a reader must not miss when interpreting the
	// results, e.g. an important check that was skipped. It appears as a
	// dedicated top-level `warnings` section in the report and is also
	// re-emitted to the log so it is not overlooked.
	Warnings        []string `json:"warnings,omitempty"`
	EnvironmentInfo `json:"environmentInfo"`
	Configuration   `json:"inputConfiguration"`
	Results         []Result `json:"results"`
}

type Result struct {
	Name   string      `json:"name"`
	Status CheckStatus `json:"status"`
	Info   string      `json:"info,omitempty"`
}

type EnvironmentInfo struct {
	HarvesterVersion string `json:"harvesterVersion"`
	NodeCount        int    `json:"nodeCount"`
	ValidatorVersion string `json:"validatorVersion"`
}

type CheckStatus string

const CheckStatusSuccess CheckStatus = "success"
const CheckStatusFailure CheckStatus = "failure"
const CheckStatusSkipped CheckStatus = "skipped"

func (r *Result) AddFailureInfo(err error) {
	r.Status = CheckStatusFailure
	r.Info = err.Error()
}
