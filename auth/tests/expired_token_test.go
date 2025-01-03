package tests_test

import (
	"mzhn/auth/tests/suite"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpiredToken(t *testing.T) {
	_, st := suite.New(t)

	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJJZCI6IjMzZTdlMmJlLTk3ZWItNGI1ZC05NmVmLWMyZTFhMjQ3NmQwYiIsIkVtYWlsIjoicmV3a2FzaEB2ay5jb20iLCJleHAiOjE3MjkyNjQ4ODYsImlhdCI6MTcyOTI2NDg4Nn0.fwXzepfiNlONOZMLpPyL36pbHHrRyIWBGDmJkdwei7Q"

	respProfile, dataProfile := st.Profile(t, token)
	require.Equal(t, http.StatusUnauthorized, respProfile.StatusCode)
	assert.NotEmpty(t, dataProfile["error"])
}
