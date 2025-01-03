package tests_test

import (
	"mzhn/auth/tests/suite"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefresh(t *testing.T) {
	_, st := suite.New(t)

	email, pass := genuser()

	respReg, dataReg := st.Register(t, email, pass)
	require.Equal(t, http.StatusOK, respReg.StatusCode)
	rt, ok := dataReg["refreshToken"].(string)
	require.True(t, ok)

	assert.NotEmpty(t, rt)
	assert.NotEmpty(t, dataReg["accessToken"].(string))

	respRefresh, dataRefresh := st.Refresh(t, rt)
	require.Equal(t, http.StatusOK, respRefresh.StatusCode)
	assert.NotEmpty(t, dataRefresh["accessToken"].(string))
	assert.NotEmpty(t, dataRefresh["refreshToken"].(string))
}
