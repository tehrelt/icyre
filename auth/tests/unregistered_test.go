package tests_test

import (
	"mzhn/auth/tests/suite"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnregisteredUser(t *testing.T) {
	_, st := suite.New(t)

	email, pass := "unregistered@mail.ru", "password"

	respLog, dataLog := st.Login(t, email, pass)
	require.Equal(t, http.StatusBadRequest, respLog.StatusCode)
	assert.NotEmpty(t, dataLog["error"])
}
