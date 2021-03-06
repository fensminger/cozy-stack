package errors

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/cozy/cozy-stack/pkg/config"
	"github.com/cozy/cozy-stack/pkg/couchdb"
	"github.com/cozy/cozy-stack/pkg/instance"
	"github.com/cozy/cozy-stack/pkg/logger"
	"github.com/cozy/cozy-stack/web/jsonapi"
	"github.com/cozy/cozy-stack/web/middlewares"

	"github.com/cozy/echo"
	"github.com/sirupsen/logrus"
)

// ErrorHandler is the default error handler of our APIs.
func ErrorHandler(err error, c echo.Context) {
	var je *jsonapi.Error
	var ce *couchdb.Error

	res := c.Response()
	req := c.Request()

	var ok bool
	if _, ok = err.(*echo.HTTPError); ok {
		// nothing to do
	} else if os.IsExist(err) {
		je = jsonapi.Conflict(err)
	} else if os.IsNotExist(err) {
		je = jsonapi.NotFound(err)
	} else if ce, ok = err.(*couchdb.Error); ok {
		je = &jsonapi.Error{
			Status: ce.StatusCode,
			Title:  ce.Name,
			Detail: ce.Reason,
		}
	} else if je, ok = err.(*jsonapi.Error); !ok {
		je = &jsonapi.Error{
			Status: http.StatusInternalServerError,
			Title:  "Unqualified error",
			Detail: err.Error(),
		}
	}

	if config.IsDevRelease() {
		var log *logrus.Entry
		inst, ok := c.Get("instance").(*instance.Instance)
		if ok {
			log = inst.Logger().WithField("nspace", "http")
		} else {
			log = logger.WithNamespace("http")
		}
		log.Errorf("%s %s %s", req.Method, req.URL.Path, err)
	}

	if res.Committed {
		return
	}

	if je != nil {
		if req.Method == http.MethodHead {
			c.NoContent(je.Status)
			return
		}
		jsonapi.DataError(c, je)
		return
	}

	HTMLErrorHandler(err, c)
}

// HTMLErrorHandler is the default fallback error handler for error rendered in
// HTML pages, mainly for users, assets and routes that are not part of our API
// per-se.
func HTMLErrorHandler(err error, c echo.Context) {
	status := http.StatusInternalServerError

	req := c.Request()

	var log *logrus.Entry
	inst, ok := c.Get("instance").(*instance.Instance)
	if ok {
		log = inst.Logger().WithField("nspace", "http")
	} else {
		log = logger.WithNamespace("http")
	}
	log.Errorf("%s %s %s", req.Method, req.URL.Path, err)

	var he *echo.HTTPError
	if he, ok = err.(*echo.HTTPError); ok {
		status = he.Code
		if he.Inner != nil {
			err = he.Inner
		}
	} else {
		he = echo.NewHTTPError(status, err)
		he.Inner = err
	}

	var title, value string
	if err == instance.ErrNotFound {
		status = http.StatusNotFound
		title = "Error Instance not found Title"
		value = "Error Instance not found Message"
	}
	if title == "" {
		if status >= 500 {
			title = "Error Internal Server Error Title"
			value = "Error Internal Server Error Message"
		} else {
			title = "Error Title"
			value = fmt.Sprintf("%v", he.Message)
		}
	}

	accept := req.Header.Get("Accept")
	acceptHTML := strings.Contains(accept, echo.MIMETextHTML)
	acceptJSON := strings.Contains(accept, echo.MIMEApplicationJSON)
	if req.Method == http.MethodHead {
		err = c.NoContent(status)
	} else if acceptJSON {
		err = c.JSON(status, echo.Map{"error": he.Message})
	} else if acceptHTML {
		var domain string
		i, ok := middlewares.GetInstanceSafe(c)
		if ok {
			domain = i.Domain
		}
		err = c.Render(status, "error.html", echo.Map{
			"Domain":     domain,
			"ErrorTitle": title,
			"Error":      value,
		})
	} else {
		err = c.String(status, fmt.Sprintf("%v", he.Message))
	}

	if err != nil && log != nil {
		log.Errorf("%s %s %s", req.Method, req.URL.Path, err)
	}
}
