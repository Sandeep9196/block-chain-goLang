package http

import (
	"net/http"
	"testing"

	"bsc_network/internal/mocks"
	"bsc_network/internal/test"
	"bsc_network/internal/config/log"
)

func TestAPI(t *testing.T) {
	l, _ := log.NewForTest()
	logger := log.NewWithZap(l)

	router := mocks.Router(logger)

	RegisterHandlers(router.Group(""), "test")

	tests := []test.APITestCase{
		{
			Name:         "get",
			Method:       http.MethodGet,
			URL:          "/healthcheck",
			WantStatus:   http.StatusOK,
			WantResponse: `*OK test*`,
		},
	}

	for _, tc := range tests {
		test.Endpoint(t, router, tc)
	}
}
