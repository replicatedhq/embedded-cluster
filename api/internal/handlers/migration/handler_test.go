package migration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockController is a mock implementation of migration.Controller
type MockController struct {
	mock.Mock
}

func (m *MockController) GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.LinuxInstallationConfigResponse), args.Error(1)
}

func (m *MockController) StartMigration(ctx context.Context, transferMode types.TransferMode, config types.LinuxInstallationConfig) (string, error) {
	args := m.Called(ctx, transferMode, config)
	return args.String(0), args.Error(1)
}

func (m *MockController) GetMigrationStatus(ctx context.Context) (types.MigrationStatusResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.MigrationStatusResponse), args.Error(1)
}

func (m *MockController) Run(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestGetInstallationConfig(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*MockController)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful response",
			setupMock: func(mc *MockController) {
				mc.On("GetInstallationConfig", mock.Anything).Return(
					types.LinuxInstallationConfigResponse{
						Values: types.LinuxInstallationConfig{
							PodCIDR:     "10.244.0.0/16",
							ServiceCIDR: "10.96.0.0/12",
						},
						Defaults: types.LinuxInstallationConfig{
							PodCIDR:     "10.244.0.0/16",
							ServiceCIDR: "10.96.0.0/12",
						},
						Resolved: types.LinuxInstallationConfig{
							PodCIDR:     "10.244.0.0/16",
							ServiceCIDR: "10.96.0.0/12",
						},
					},
					nil,
				)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response types.LinuxInstallationConfigResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "10.244.0.0/16", response.Values.PodCIDR)
				assert.Equal(t, "10.96.0.0/12", response.Values.ServiceCIDR)
			},
		},
		{
			name: "controller error",
			setupMock: func(mc *MockController) {
				mc.On("GetInstallationConfig", mock.Anything).Return(
					types.LinuxInstallationConfigResponse{},
					fmt.Errorf("controller error"),
				)
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var apiErr types.APIError
				err := json.Unmarshal(rec.Body.Bytes(), &apiErr)
				require.NoError(t, err)
				assert.Contains(t, apiErr.Message, "controller error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockController := &MockController{}
			tt.setupMock(mockController)

			handler := New(
				WithController(mockController),
				WithLogger(logger.NewDiscardLogger()),
			)

			req := httptest.NewRequest(http.MethodGet, "/migration/config", nil)
			rec := httptest.NewRecorder()

			handler.GetInstallationConfig(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			tt.checkResponse(t, rec)
			mockController.AssertExpectations(t)
		})
	}
}

func TestPostStartMigration(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(*MockController)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful start with copy mode",
			requestBody: types.StartMigrationRequest{
				TransferMode: types.TransferModeCopy,
				Config: &types.LinuxInstallationConfig{
					PodCIDR:     "10.244.0.0/16",
					ServiceCIDR: "10.96.0.0/12",
				},
			},
			setupMock: func(mc *MockController) {
				mc.On("StartMigration", mock.Anything, types.TransferModeCopy, mock.MatchedBy(func(cfg types.LinuxInstallationConfig) bool {
					return cfg.PodCIDR == "10.244.0.0/16" && cfg.ServiceCIDR == "10.96.0.0/12"
				})).Return("550e8400-e29b-41d4-a716-446655440000", nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response types.StartMigrationResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", response.MigrationID)
				assert.Equal(t, "migration started successfully", response.Message)
			},
		},
		{
			name: "successful start with move mode",
			requestBody: types.StartMigrationRequest{
				TransferMode: types.TransferModeMove,
				Config:       nil,
			},
			setupMock: func(mc *MockController) {
				mc.On("StartMigration", mock.Anything, types.TransferModeMove, types.LinuxInstallationConfig{}).Return("test-uuid", nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response types.StartMigrationResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "test-uuid", response.MigrationID)
			},
		},
		{
			name: "default to copy when mode is empty",
			requestBody: types.StartMigrationRequest{
				TransferMode: "",
				Config:       nil,
			},
			setupMock: func(mc *MockController) {
				mc.On("StartMigration", mock.Anything, types.TransferModeCopy, types.LinuxInstallationConfig{}).Return("test-uuid", nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response types.StartMigrationResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "test-uuid", response.MigrationID)
			},
		},
		{
			name:           "invalid request body",
			requestBody:    "invalid json",
			setupMock:      func(mc *MockController) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var apiErr types.APIError
				err := json.Unmarshal(rec.Body.Bytes(), &apiErr)
				require.NoError(t, err)
				assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
			},
		},
		{
			name: "migration already started",
			requestBody: types.StartMigrationRequest{
				TransferMode: types.TransferModeCopy,
				Config:       nil,
			},
			setupMock: func(mc *MockController) {
				mc.On("StartMigration", mock.Anything, types.TransferModeCopy, types.LinuxInstallationConfig{}).Return("", types.NewConflictError(types.ErrMigrationAlreadyStarted))
			},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var apiErr types.APIError
				err := json.Unmarshal(rec.Body.Bytes(), &apiErr)
				require.NoError(t, err)
				assert.Equal(t, http.StatusConflict, apiErr.StatusCode)
				assert.Contains(t, apiErr.Message, "migration already started")
			},
		},
		{
			name: "invalid transfer mode",
			requestBody: types.StartMigrationRequest{
				TransferMode: "invalid",
				Config:       nil,
			},
			setupMock:      func(mc *MockController) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var apiErr types.APIError
				err := json.Unmarshal(rec.Body.Bytes(), &apiErr)
				require.NoError(t, err)
				assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
				assert.Contains(t, apiErr.Message, "invalid transfer mode")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockController := &MockController{}
			tt.setupMock(mockController)

			handler := New(
				WithController(mockController),
				WithLogger(logger.NewDiscardLogger()),
			)

			var body []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/migration/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.PostStartMigration(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			tt.checkResponse(t, rec)
			mockController.AssertExpectations(t)
		})
	}
}

func TestGetMigrationStatus(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*MockController)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful response with active migration",
			setupMock: func(mc *MockController) {
				mc.On("GetMigrationStatus", mock.Anything).Return(
					types.MigrationStatusResponse{
						State:    types.MigrationStateInProgress,
						Phase:    types.MigrationPhaseDiscovery,
						Message:  "Discovering kURL cluster configuration",
						Progress: 25,
						Error:    "",
					},
					nil,
				)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response types.MigrationStatusResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, types.MigrationStateInProgress, response.State)
				assert.Equal(t, types.MigrationPhaseDiscovery, response.Phase)
				assert.Equal(t, 25, response.Progress)
			},
		},
		{
			name: "no active migration",
			setupMock: func(mc *MockController) {
				mc.On("GetMigrationStatus", mock.Anything).Return(
					types.MigrationStatusResponse{},
					types.NewNotFoundError(types.ErrNoActiveMigration),
				)
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var apiErr types.APIError
				err := json.Unmarshal(rec.Body.Bytes(), &apiErr)
				require.NoError(t, err)
				assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
				assert.Contains(t, apiErr.Message, "no active migration")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockController := &MockController{}
			tt.setupMock(mockController)

			handler := New(
				WithController(mockController),
				WithLogger(logger.NewDiscardLogger()),
			)

			req := httptest.NewRequest(http.MethodGet, "/migration/status", nil)
			rec := httptest.NewRecorder()

			handler.GetMigrationStatus(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			tt.checkResponse(t, rec)
			mockController.AssertExpectations(t)
		})
	}
}
