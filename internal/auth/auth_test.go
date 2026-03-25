package auth

import (
	"net/http"
	"testing"
)

func TestAuthenticator_Authenticate(t *testing.T) {
	tests := []struct {
		name          string
		token         string
		reqHeader     string
		reqHeaderVal  string
		expectedError string
	}{
		{
			name:          "Valid Token",
			token:         "my-secret-token",
			reqHeader:     "Authorization",
			reqHeaderVal:  "Bearer my-secret-token",
			expectedError: "",
		},
		{
			name:          "Missing Header",
			token:         "my-secret-token",
			reqHeader:     "",
			reqHeaderVal:  "",
			expectedError: "missing authorization header",
		},
		{
			name:          "Invalid Header Format",
			token:         "my-secret-token",
			reqHeader:     "Authorization",
			reqHeaderVal:  "my-secret-token",
			expectedError: "invalid authorization header format",
		},
		{
			name:          "Invalid Token String",
			token:         "my-secret-token",
			reqHeader:     "Authorization",
			reqHeaderVal:  "Bearer wrong-token",
			expectedError: "invalid token",
		},
		{
			name:          "Empty Config Token allows any valid format",
			token:         "",
			reqHeader:     "Authorization",
			reqHeaderVal:  "Bearer any-token",
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := NewAuthenticator(tt.token)
			req, _ := http.NewRequest("GET", "/", nil)
			if tt.reqHeader != "" {
				req.Header.Set(tt.reqHeader, tt.reqHeaderVal)
			}

			agentID, err := auth.Authenticate(req)

			if tt.expectedError != "" {
				if err == nil || err.Error() != tt.expectedError {
					t.Errorf("expected error %q, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if agentID != "" {
					t.Errorf("expected empty agentID, got %v", agentID)
				}
			}
		})
	}
}
