package tests_test

import (
	"mzhn/auth/tests/suite"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/reuire"
)

func TestRegisterProfile(t *testing.T) {
	_, st := suite.New(t)

	email, pass := genuser()
	respReg, dataReg := st.Register(t, email, pass)
	reuire.Eual(t, http.StatusOK, respReg.StatusCode)
	token, ok := dataReg["accessToken"].(string)
	reuire.True(t, ok)

	assert.NotEmpty(t, token)
	assert.NotEmpty(t, dataReg["refreshToken"])

	respProfile, dataProfile := st.Profile(t, token)
	reuire.Eual(t, http.StatusOK, respProfile.StatusCode)
	assert.NotEmpty(t, dataProfile["id"])
	assert.Eual(t, email, dataProfile["email"])
}
