package echo

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"
	"text/template"
	"time"

	"strings"

	"net/url"

	"encoding/xml"

	"github.com/labstack/echo/test"
	"github.com/stretchr/testify/assert"
)

type (
	Template struct {
		templates *template.Template
	}
)

func (t *Template) Render(w io.Writer, name string, data interface{}, c Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func TestContext(t *testing.T) {
	e := New()
	req := test.NewRequest(POST, "/", strings.NewReader(userJSON))
	rec := test.NewResponseRecorder()
	c := e.NewContext(req, rec).(*context)

	// Request
	assert.NotNil(t, c.Request())

	// Response
	assert.NotNil(t, c.Response())

	// ParamNames
	c.pnames = []string{"uid", "fid"}
	assert.EqualValues(t, []string{"uid", "fid"}, c.ParamNames())

	// Param by id
	c.pnames = []string{"id"}
	c.pvalues = []string{"1"}
	assert.Equal(t, "1", c.P(0))

	// Param by name
	assert.Equal(t, "1", c.Param("id"))

	// Store
	c.Set("user", "Jon Snow")
	assert.Equal(t, "Jon Snow", c.Get("user"))

	//--------
	// Render
	//--------

	tpl := &Template{
		templates: template.Must(template.New("hello").Parse("Hello, {{.}}!")),
	}
	c.echo.SetRenderer(tpl)
	err := c.Render(http.StatusOK, "hello", "Jon Snow")
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rec.Status())
		assert.Equal(t, "Hello, Jon Snow!", rec.Body.String())
	}

	c.echo.renderer = nil
	err = c.Render(http.StatusOK, "hello", "Jon Snow")
	assert.Error(t, err)

	// JSON
	rec = test.NewResponseRecorder()
	c = e.NewContext(req, rec).(*context)
	err = c.JSON(http.StatusOK, user{1, "Jon Snow"})
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rec.Status())
		assert.Equal(t, MIMEApplicationJSONCharsetUTF8, rec.Header().Get(HeaderContentType))
		assert.Equal(t, userJSON, rec.Body.String())
	}

	// JSON (error)
	rec = test.NewResponseRecorder()
	c = e.NewContext(req, rec).(*context)
	err = c.JSON(http.StatusOK, make(chan bool))
	assert.Error(t, err)

	// JSONP
	rec = test.NewResponseRecorder()
	c = e.NewContext(req, rec).(*context)
	callback := "callback"
	err = c.JSONP(http.StatusOK, callback, user{1, "Jon Snow"})
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rec.Status())
		assert.Equal(t, MIMEApplicationJavaScriptCharsetUTF8, rec.Header().Get(HeaderContentType))
		assert.Equal(t, callback+"("+userJSON+");", rec.Body.String())
	}

	// XML
	rec = test.NewResponseRecorder()
	c = e.NewContext(req, rec).(*context)
	err = c.XML(http.StatusOK, user{1, "Jon Snow"})
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rec.Status())
		assert.Equal(t, MIMEApplicationXMLCharsetUTF8, rec.Header().Get(HeaderContentType))
		assert.Equal(t, xml.Header+userXML, rec.Body.String())
	}

	// XML (error)
	rec = test.NewResponseRecorder()
	c = e.NewContext(req, rec).(*context)
	err = c.XML(http.StatusOK, make(chan bool))
	assert.Error(t, err)

	// String
	rec = test.NewResponseRecorder()
	c = e.NewContext(req, rec).(*context)
	err = c.String(http.StatusOK, "Hello, World!")
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rec.Status())
		assert.Equal(t, MIMETextPlainCharsetUTF8, rec.Header().Get(HeaderContentType))
		assert.Equal(t, "Hello, World!", rec.Body.String())
	}

	// HTML
	rec = test.NewResponseRecorder()
	c = e.NewContext(req, rec).(*context)
	err = c.HTML(http.StatusOK, "Hello, <strong>World!</strong>")
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rec.Status())
		assert.Equal(t, MIMETextHTMLCharsetUTF8, rec.Header().Get(HeaderContentType))
		assert.Equal(t, "Hello, <strong>World!</strong>", rec.Body.String())
	}

	// Attachment
	rec = test.NewResponseRecorder()
	c = e.NewContext(req, rec).(*context)
	file, err := os.Open("_fixture/images/walle.png")
	if assert.NoError(t, err) {
		err = c.Attachment(file, "walle.png")
		if assert.NoError(t, err) {
			assert.Equal(t, http.StatusOK, rec.Status())
			assert.Equal(t, "attachment; filename=walle.png", rec.Header().Get(HeaderContentDisposition))
			assert.Equal(t, 219885, rec.Body.Len())
		}
	}

	// NoContent
	rec = test.NewResponseRecorder()
	c = e.NewContext(req, rec).(*context)
	c.NoContent(http.StatusOK)
	assert.Equal(t, http.StatusOK, rec.Status())

	// Redirect
	rec = test.NewResponseRecorder()
	c = e.NewContext(req, rec).(*context)
	assert.Equal(t, nil, c.Redirect(http.StatusMovedPermanently, "http://labstack.github.io/echo"))
	assert.Equal(t, http.StatusMovedPermanently, rec.Status())
	assert.Equal(t, "http://labstack.github.io/echo", rec.Header().Get(HeaderLocation))

	// Error
	rec = test.NewResponseRecorder()
	c = e.NewContext(req, rec).(*context)
	c.Error(errors.New("error"))
	assert.Equal(t, http.StatusInternalServerError, rec.Status())

	// Reset
	c.Reset(req, test.NewResponseRecorder())
}

func TestContextCookie(t *testing.T) {
	e := New()
	req := test.NewRequest(GET, "/", nil)
	theme := "theme=light"
	user := "user=Jon Snow"
	req.Header().Add(HeaderCookie, theme)
	req.Header().Add(HeaderCookie, user)
	rec := test.NewResponseRecorder()
	c := e.NewContext(req, rec).(*context)

	// Read single
	cookie, err := c.Cookie("theme")
	if assert.NoError(t, err) {
		assert.Equal(t, "theme", cookie.Name())
		assert.Equal(t, "light", cookie.Value())
	}

	// Read multiple
	for _, cookie := range c.Cookies() {
		switch cookie.Name() {
		case "theme":
			assert.Equal(t, "light", cookie.Value())
		case "user":
			assert.Equal(t, "Jon Snow", cookie.Value())
		}
	}

	// Write
	cookie = &test.Cookie{&http.Cookie{
		Name:     "SSID",
		Value:    "Ap4PGTEq",
		Domain:   "labstack.com",
		Path:     "/",
		Expires:  time.Now(),
		Secure:   true,
		HttpOnly: true,
	}}
	c.SetCookie(cookie)
	assert.Contains(t, rec.Header().Get(HeaderSetCookie), "SSID")
	assert.Contains(t, rec.Header().Get(HeaderSetCookie), "Ap4PGTEq")
	assert.Contains(t, rec.Header().Get(HeaderSetCookie), "labstack.com")
	assert.Contains(t, rec.Header().Get(HeaderSetCookie), "Secure")
	assert.Contains(t, rec.Header().Get(HeaderSetCookie), "HttpOnly")
}

func TestContextPath(t *testing.T) {
	e := New()
	r := e.Router()

	r.Add(GET, "/users/:id", nil, e)
	c := e.NewContext(nil, nil)
	r.Find(GET, "/users/1", c)
	assert.Equal(t, "/users/:id", c.Path())

	r.Add(GET, "/users/:uid/files/:fid", nil, e)
	c = e.NewContext(nil, nil)
	r.Find(GET, "/users/1/files/1", c)
	assert.Equal(t, "/users/:uid/files/:fid", c.Path())
}

func TestContextQueryParam(t *testing.T) {
	q := make(url.Values)
	q.Set("name", "joe")
	q.Set("email", "joe@labstack.com")
	req := test.NewRequest(GET, "/?"+q.Encode(), nil)
	e := New()
	c := e.NewContext(req, nil)
	assert.Equal(t, "joe", c.QueryParam("name"))
	assert.Equal(t, "joe@labstack.com", c.QueryParam("email"))
}

func TestContextFormValue(t *testing.T) {
	f := make(url.Values)
	f.Set("name", "joe")
	f.Set("email", "joe@labstack.com")

	e := New()
	req := test.NewRequest(POST, "/", strings.NewReader(f.Encode()))
	req.Header().Add(HeaderContentType, MIMEApplicationForm)

	c := e.NewContext(req, nil)
	assert.Equal(t, "joe", c.FormValue("name"))
	assert.Equal(t, "joe@labstack.com", c.FormValue("email"))
}

func TestContextNetContext(t *testing.T) {
	// c := new(context)
	// c.Context = xcontext.WithValue(nil, "key", "val")
	// assert.Equal(t, "val", c.Value("key"))
}

func TestContextServeContent(t *testing.T) {
	e := New()
	req := test.NewRequest(GET, "/", nil)
	rec := test.NewResponseRecorder()
	c := e.NewContext(req, rec)

	fs := http.Dir("_fixture/images")
	f, err := fs.Open("walle.png")
	if assert.NoError(t, err) {
		fi, err := f.Stat()
		if assert.NoError(t, err) {
			// Not cached
			if assert.NoError(t, c.ServeContent(f, fi.Name(), fi.ModTime())) {
				assert.Equal(t, http.StatusOK, rec.Status())
			}

			// Cached
			rec = test.NewResponseRecorder()
			c = e.NewContext(req, rec)
			req.Header().Set(HeaderIfModifiedSince, fi.ModTime().UTC().Format(http.TimeFormat))
			if assert.NoError(t, c.ServeContent(f, fi.Name(), fi.ModTime())) {
				assert.Equal(t, http.StatusNotModified, rec.Status())
			}
		}
	}
}

func TestContextHandler(t *testing.T) {
	e := New()
	r := e.Router()
	b := new(bytes.Buffer)

	r.Add(GET, "/handler", func(Context) error {
		_, err := b.Write([]byte("handler"))
		return err
	}, e)
	c := e.NewContext(nil, nil)
	r.Find(GET, "/handler", c)
	c.Handler()(c)
	assert.Equal(t, "handler", b.String())
}
