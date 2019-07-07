package apitest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/teejays/n-factor-vault/backend/library/go-api"
)

/* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
* T E S T   S U I T E
* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */

// TestSuite defines a configuration that wraps a bunch of individual tests for a single HandlerFunc
type TestSuite struct {
	Route          string
	Method         string
	HandlerFunc    http.HandlerFunc
	AfterTestFunc  func(*testing.T)
	BeforeTestFunc func(*testing.T)
}

// HandlerTest defines configuration for a single test run for a HandlerFunc. It is run run as part of the TestSuite
type HandlerTest struct {
	Name                string
	Content             string
	WantStatusCode      int
	WantContent         string
	WantErr             bool
	WantErrMessage      string
	AssertContentFields map[string]AssertFunc
	BeforeRunFunc       func(*testing.T)
	AfterRunFunc        func(*testing.T)
	SkipBeforeTestFunc  bool
	SkipAfterTestFunc   bool
}

// RunHandlerTests runs all the HandlerTests inside a testing.T.Run() loop
func (ts TestSuite) RunHandlerTests(t *testing.T, tests []HandlerTest) {
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			ts.RunHandlerTest(t, tt)
		})
	}
}

// RunHandlerTest run all the HandlerTest tt
func (ts TestSuite) RunHandlerTest(t *testing.T, tt HandlerTest) {

	// Run BeforeRunFuncs
	if ts.BeforeTestFunc != nil && !tt.SkipBeforeTestFunc {
		ts.BeforeTestFunc(t)
	}

	if tt.BeforeRunFunc != nil {
		tt.BeforeRunFunc(t)
	}

	// Create the HTTP request and response
	hreq := HandlerReqParams{
		ts.Route,
		ts.Method,
		ts.HandlerFunc,
	}
	resp, body, err := MakeHandlerRequest(hreq, tt.Content, []int{tt.WantStatusCode})
	assert.NoError(t, err)

	// Verify the respoonse
	assert.Equal(t, tt.WantStatusCode, resp.StatusCode)

	if tt.WantContent != "" {
		assert.Equal(t, tt.WantContent, string(body))
	}

	if tt.WantErrMessage != "" || tt.WantErr {
		var errH api.Error
		err = json.Unmarshal(body, &errH)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, tt.WantStatusCode, int(errH.Code))

		if tt.WantErr {
			assert.NotEmpty(t, errH.Message)
		}

		if tt.WantErrMessage != "" {
			assert.Contains(t, errH.Message, tt.WantErrMessage)
		}

	}

	// Run the individual assert functions for each of the field in the HTTP response body
	if tt.AssertContentFields != nil {
		// Unmarshall the body in to a map[string]interface{}
		var rJSON = make(map[string]interface{})
		err = json.Unmarshal(body, &rJSON)
		if err != nil {
			t.Error(err)
		}
		// Loop over all the available assert funcs specified and run them for the given field
		for k, assertFunc := range tt.AssertContentFields {
			v, exists := rJSON[k]
			if !exists {
				t.Errorf("the key '%s' does not exist in the response but an AssertFunc for it was specified", k)
			}
			assertFunc(t, v)
		}
	}

	// Run AfterRunFuncs
	if tt.AfterRunFunc != nil {
		tt.AfterRunFunc(t)
	}

	if ts.AfterTestFunc != nil && !tt.SkipAfterTestFunc {
		ts.AfterTestFunc(t)
	}

}

/* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
* A S S E R T   F U N C S
* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */

// AssertFunc is a function that takes the testing.T pointer, a value v, and asserts
// whether v is good
type AssertFunc func(t *testing.T, v interface{})

// AssertIsEqual is a of type AssertFunc. It verifies that the value v is equal to the expected value.
var AssertIsEqual = func(expected interface{}) AssertFunc {
	return func(t *testing.T, v interface{}) {
		assert.Equal(t, expected, v)
	}
}

// AssertNotEmptyFunc is a of type AssertFunc. It verifies that the value v is not empty.
var AssertNotEmptyFunc = func(t *testing.T, v interface{}) {
	assert.NotEmpty(t, v)
}

/* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
* H A N D L E R   R E Q U E S T
* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */

// HandlerReqParams define a set of configuration that allow us to make repeated calls to Handler
type HandlerReqParams struct {
	Route       string
	Method      string
	HandlerFunc http.HandlerFunc
	// Content             string
	// AcceptedStatusCodes []int
}

// MakeHandlerRequest makes an request to the handler specified in p, using the content. It errors if there is an
// error making the request, or if the received status code is not among the accepted status codes
func MakeHandlerRequest(p HandlerReqParams, content string, acceptedStatusCodes []int) (*http.Response, []byte, error) {
	// Create the HTTP request and response
	var buff = bytes.NewBufferString(content)
	var r = httptest.NewRequest(p.Method, p.Route, buff)
	var w = httptest.NewRecorder()

	// Call the Handler
	p.HandlerFunc(w, r)

	resp := w.Result()

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, body, err
	}

	// Check if the response status is one of the accepted ones
	if len(acceptedStatusCodes) > 0 {
		var statusMap = make(map[int]bool)
		for _, status := range acceptedStatusCodes {
			statusMap[status] = true
		}
		if v, hasKey := statusMap[w.Code]; !hasKey || !v {
			return resp, body, fmt.Errorf("apitest: handler request to %s resulted in a unaccepteable %d status:\n%s", p.Route, w.Code, string(body))
		}
	}

	return resp, body, nil
}
