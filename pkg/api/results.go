package api

type Report struct {
	Configuration `json:"inputConfiguration"`
	Results       []Result `json:"results"`
}

type Result struct {
	Name   string      `json:"name"`
	Status CheckStatus `json:"status"`
	Info   string      `json:"info,omitempty"`
}

type CheckStatus string

const CheckStatusSuccess CheckStatus = "success"
const CheckStatusFailure CheckStatus = "failure"
const CheckStatusSkipped CheckStatus = "skipped"

func (r *Result) AddFailureInfo(err error) {
	r.Status = CheckStatusFailure
	r.Info = err.Error()
}
