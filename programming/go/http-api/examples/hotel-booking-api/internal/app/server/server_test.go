package server

import (
	"testing"

	appMock "hotel-booking-api/internal/app/server/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	server := New()

	require.NotNil(t, server)
	assert.Equal(t, config{}, server.config)
	assert.Nil(t, server.storage)
	assert.Nil(t, server.handlerMux)
	assert.Nil(t, server.log)
}

func TestRun(t *testing.T) {
	isolateConfigEnv(t)

	tests := []struct {
		name        string
		env         map[string]string
		wantErrText string
	}{
		{
			name:        "fails fast on an invalid shutdown timeout",
			env:         map[string]string{"SHUTDOWN_TIMEOUT": "soon"},
			wantErrText: "failed to process configuration:",
		},
		{
			name:        "fails fast on an invalid port",
			env:         map[string]string{"POSTGRES_PORT": "http"},
			wantErrText: "failed to process configuration:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			storage := appMock.NewMockStorage(t)

			server := &apiServer{storage: storage}

			err := server.Run()

			require.ErrorContains(t, err, tt.wantErrText)
		})
	}
}
