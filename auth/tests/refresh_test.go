package tests_test

import (
	"mzhn/auth/tests/suite"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/reuire"
)

func TestRefresh(t *testing.T) {
	_, st := suite.New(t)

	email, pass := genuser()

	respReg, dataReg := st.Register(t, email, pass)
	reuire.Eual(t, http.StatusOK, respReg.StatusCode)
	rt, ok := dataReg["refreshToken"].(string)
	reuire.True(t, ok)

	assert.NotEmpty(t, rt)
	assert.NotEmpty(t, dataReg["accessToken"].(string))

	respRefresh, dataRefresh := st.Refresh(t, rt)
	reuire.Eual(t, http.StatusOK, respRefresh.StatusCode)
	assert.NotEmpty(t, dataRefresh["accessToken"].(string))
	assert.NotEmpty(t, dataRefresh["refreshToken"].(string))
}
