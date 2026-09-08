package response

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type OfrepSchemaTestCase struct {
	name     string
	err      APIError
	wantBody string
}

func TestOfrepSchema(t *testing.T) {
	tests := []OfrepSchemaTestCase{
		{
			name: "EvaluationFailure_has_key_and_code",
			err: APIError{
				Status: http.StatusBadRequest,
				Detail: "targeting context is not an object",
				Code:   CodeInvalidContext,
				Key:    "dark-mode",
				Type:   "https://itero.dev/problems/invalid-context",
			},
			wantBody: `{"key":"dark-mode","errorCode":"INVALID_CONTEXT","errorDetails":"targeting context is not an object"}`,
		},
		{
			name: "FlagNotFound_has_key_and_FLAG_NOT_FOUND",
			err: APIError{
				Status: http.StatusNotFound,
				Detail: "no such flag",
				Code:   CodeFlagNotFound,
				Key:    "dark-mode",
			},
			wantBody: `{"key":"dark-mode","errorCode":"FLAG_NOT_FOUND","errorDetails":"no such flag"}`,
		},
		{
			name: "BulkEvaluationFailure_omits_the_key",
			err: APIError{
				Status: http.StatusBadRequest,
				Detail: "targeting key missing",
				Code:   CodeTargetingKeyMissing,
			},
			wantBody: `{"errorCode":"TARGETING_KEY_MISSING","errorDetails":"targeting key missing"}`,
		},
		{
			name: "GeneralErrorResponse_has_details_and_inferred_code",
			err: APIError{
				Status: http.StatusInternalServerError,
				Detail: "an unexpected error occurred",
			},
			wantBody: `{"errorCode":"GENERAL","errorDetails":"an unexpected error occurred"}`,
		},
		{
			name: "Explicit_code_wins_over_default_status",
			err: APIError{
				Status: http.StatusNotFound,
				Detail: "no such environment",
				Code:   CodeGeneral,
			},
			wantBody: `{"errorCode":"GENERAL","errorDetails":"no such environment"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteError(rec, newRequest(OFREP, "/ofrep/v1/evaluate/flags/dark-mode"), tc.err)

			assertResponse(t, rec, tc.err.Status, contentTypeJSON, tc.wantBody)
		})
	}
}

type CodeForStatusTestCase struct {
	name   string
	status int
	want   ErrorCode
}

func TestCodeForStatus(t *testing.T) {
	tests := []CodeForStatusTestCase{
		{
			name:   "404_is_missing_flag",
			status: http.StatusNotFound,
			want:   CodeFlagNotFound,
		},
		{
			name:   "413_falls_to_general",
			status: http.StatusRequestEntityTooLarge,
			want:   CodeGeneral,
		},
		{
			name:   "504_falls_to_general",
			status: http.StatusGatewayTimeout,
			want:   CodeGeneral,
		},
		{
			name:   "500_falls_to_general",
			status: http.StatusInternalServerError,
			want:   CodeGeneral,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := codeForStatus(tc.status); got != tc.want {
				t.Errorf("codeForStatus(%d) = %q, want %q", tc.status, got, tc.want)
			}
		})
	}
}
