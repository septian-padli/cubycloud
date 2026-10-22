package organization

import (
	"net/http"
	"testing"

	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
)

func TestRespondStorageUnavailableUses503(t *testing.T) {
	err := respond(echo.New().NewContext(nil, nil), storage.ErrUnavailable)
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusServiceUnavailable {
		t.Fatalf("got response error %#v, want HTTP 503", err)
	}
}
