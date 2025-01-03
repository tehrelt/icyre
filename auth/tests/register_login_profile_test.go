package tests_test

import (
	"mzhn/auth/tests/suite"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterLoginGood(t *testing.T) {

	_, st := suite.New(t)

	email, pass := genuser()

	respReg, dataReg := st.Register(t, email, pass)
	require.Equal(t, http.StatusOK, respReg.StatusCode)
	assert.NotEmpty(t, dataReg["accessToken"])
	assert.NotEmpty(t, dataReg["refreshToken"])

	respLog, dataLog := st.Login(t, email, pass)
	require.Equal(t, http.StatusOK, respLog.StatusCode)
	assert.NotEmpty(t, dataLog["accessToken"])
	assert.NotEmpty(t, dataLog["refreshToken"])
}
