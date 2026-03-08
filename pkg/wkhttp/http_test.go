package wkhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestResponseErrorWithStatus(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		status         int
		expectedStatus int
	}{
		{
			name:           "NotFound status",
			err:            errors.New("resource not found"),
			status:         http.StatusNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "InternalServerError status",
			err:            errors.New("internal error"),
			status:         http.StatusInternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "Forbidden status",
			err:            errors.New("access denied"),
			status:         http.StatusForbidden,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "BadRequest status",
			err:            errors.New("bad request"),
			status:         http.StatusBadRequest,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Unauthorized status",
			err:            errors.New("unauthorized"),
			status:         http.StatusUnauthorized,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ginCtx, _ := gin.CreateTestContext(w)

			ctx := &Context{Context: ginCtx}
			ctx.ResponseErrorWithStatus(tt.err, tt.status)

			if w.Code != tt.expectedStatus {
				t.Errorf("ResponseErrorWithStatus() HTTP status = %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestResponseError(t *testing.T) {
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)

	ctx := &Context{Context: ginCtx}
	ctx.ResponseError(errors.New("test error"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("ResponseError() HTTP status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestResponseOK(t *testing.T) {
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)

	ctx := &Context{Context: ginCtx}
	ctx.ResponseOK()

	if w.Code != http.StatusOK {
		t.Errorf("ResponseOK() HTTP status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestResponseWithStatus(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		expectedStatus int
	}{
		{
			name:           "Created status",
			status:         http.StatusCreated,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "Accepted status",
			status:         http.StatusAccepted,
			expectedStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ginCtx, _ := gin.CreateTestContext(w)

			ctx := &Context{Context: ginCtx}
			ctx.ResponseWithStatus(tt.status, gin.H{"data": "test"})

			if w.Code != tt.expectedStatus {
				t.Errorf("ResponseWithStatus() HTTP status = %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}
