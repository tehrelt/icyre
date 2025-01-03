package tests_test

import (
	"mzhn/auth/tests/suite"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterProfile(t *testing.T) {
	_, st := suite.New(t)

	email, pass := genuser()
	respReg, dataReg := st.Register(t, email, pass)
	require.Equal(t, http.StatusOK, respReg.StatusCode)
	token, ok := dataReg["accessToken"].(string)
	require.True(t, ok)

	assert.NotEmpty(t, token)
	assert.NotEmpty(t, dataReg["refreshToken"])

	respProfile, dataProfile := st.Profile(t, token)
	require.Equal(t, http.StatusOK, respProfile.StatusCode)
	assert.NotEmpty(t, dataProfile["id"])
	assert.Equal(t, email, dataProfile["email"])
}
