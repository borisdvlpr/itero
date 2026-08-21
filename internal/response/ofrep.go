package response

import "net/http"

// ErrorCode is an OpenFeature error code. OFREP constrains which codes each
// response may carry: evaluationFailure (400) accepts PARSE_ERROR,
// TARGETING_KEY_MISSING, INVALID_CONTEXT and GENERAL, while flagNotFound (404)
// accepts only FLAG_NOT_FOUND. Nothing enforces that here, so handlers should
// set APIError.Code deliberately rather than leaning on the default.
type ErrorCode string

const (
	CodeParseError          ErrorCode = "PARSE_ERROR"
	CodeTargetingKeyMissing ErrorCode = "TARGETING_KEY_MISSING"
	CodeInvalidContext      ErrorCode = "INVALID_CONTEXT"
	CodeFlagNotFound        ErrorCode = "FLAG_NOT_FOUND"
	CodeGeneral             ErrorCode = "GENERAL"
)

// ofrepError covers all four of OFREP's error schemas. Every field is
// omitempty, so one struct serialises as evaluationFailure (key + errorCode),
// flagNotFound (key + FLAG_NOT_FOUND), bulkEvaluationFailure (errorCode, no
// key) and generalErrorResponse (errorDetails alone).
type ofrepError struct {
	Key          string    `json:"key,omitempty"`
	ErrorCode    ErrorCode `json:"errorCode,omitempty"`
	ErrorDetails string    `json:"errorDetails,omitempty"`
}

func writeOFREP(w http.ResponseWriter, r *http.Request, e APIError) {
	code := e.Code
	if code == "" {
		code = codeForStatus(e.Status)
	}

	body := ofrepError{
		Key:          e.Key,
		ErrorCode:    code,
		ErrorDetails: e.Detail,
	}

	// OFREP uses plain application/json throughout, not a +json suffix type
	write(w, r, e.Status, contentTypeJSON, body)
}

// codeForStatus supplies a code for errors raised outside a handler, where no
// more specific one is known. OFREP does not define 413 or 504 at all, and its
// 500 schema carries no errorCode field; GENERAL is the closest honest answer,
// and sending it is an additive extension a provider is free to ignore.
func codeForStatus(status int) ErrorCode {
	if status == http.StatusNotFound {
		return CodeFlagNotFound
	}

	return CodeGeneral
}
