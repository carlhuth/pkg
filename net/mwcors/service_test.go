// Copyright (c) 2014 Olivier Poitrey <rs@dailymotion.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is furnished
// to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package mwcors_test

import (
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/corestoreio/csfw/config/cfgmock"
	"github.com/corestoreio/csfw/net/mwcors"
	"github.com/corestoreio/csfw/net/mwcors/internal/corstest"
	"github.com/corestoreio/csfw/store"
	"github.com/corestoreio/csfw/store/scope"
	"github.com/corestoreio/csfw/store/storemock"
	"github.com/corestoreio/csfw/util/errors"
	"github.com/corestoreio/csfw/util/log"
	"github.com/stretchr/testify/assert"
)

func reqWithStore(method string) *http.Request {
	req, err := http.NewRequest(method, "http://corestore.io/foo", nil)
	if err != nil {
		panic(err)
	}

	return req.WithContext(
		store.WithContextRequestedStore(req.Context(), storemock.MustNewStoreAU(cfgmock.NewService())),
	)
}

func TestMustNew_Default(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			err := r.(error)
			assert.True(t, errors.IsNotValid(err), "Error: %s", err)
		} else {
			t.Fatal("Expecting a Panic")
		}
	}()
	_ = mwcors.MustNew(mwcors.WithMaxAge(scope.Default, 0, -2*time.Second))
}

func TestMustNew_Website(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			err := r.(error)
			assert.True(t, errors.IsNotValid(err), "Error: %s", err)
		} else {
			t.Fatal("Expecting a Panic")
		}
	}()
	_ = mwcors.MustNew(mwcors.WithMaxAge(scope.Website, 2, -2*time.Second))
}

func TestNoConfig(t *testing.T) {
	s := mwcors.MustNew()
	req := reqWithStore("GET")
	corstest.TestNoConfig(t, s, req)
}

func TestService_Options_Scope_Website(t *testing.T) {

	var newSrv = func(opts ...mwcors.Option) *mwcors.Service {
		s := mwcors.MustNew(mwcors.WithLogger(log.BlackHole{})) // why can't i append opts... after WithLogger() ?
		if err := s.Options(opts...); err != nil {
			t.Fatal(err)
		}
		return s
	}

	tests := []struct {
		srv    *mwcors.Service
		req    *http.Request
		tester func(t *testing.T, s *mwcors.Service, req *http.Request)
	}{
		{
			newSrv(mwcors.WithAllowedOrigins(scope.Website, 2, "*")),
			reqWithStore("GET"),
			corstest.TestMatchAllOrigin,
		},
		{
			newSrv(mwcors.WithAllowedOrigins(scope.Website, 2, "http://foobar.com")),
			reqWithStore("GET"),
			corstest.TestAllowedOrigin,
		},
		{
			newSrv(mwcors.WithAllowedOrigins(scope.Website, 2, "http://*.bar.com")),
			reqWithStore("GET"),
			corstest.TestWildcardOrigin,
		},
		{
			newSrv(mwcors.WithAllowedOrigins(scope.Website, 2, "http://foobar.com")),
			reqWithStore("GET"),
			corstest.TestDisallowedOrigin,
		},
		{
			newSrv(mwcors.WithAllowedOrigins(scope.Website, 2, "http://*.bar.com")),
			reqWithStore("GET"),
			corstest.TestDisallowedWildcardOrigin,
		},
		{
			newSrv(mwcors.WithAllowOriginFunc(scope.Website, 2, func(o string) bool {
				r, _ := regexp.Compile("^http://foo") // don't do this on production systems!
				return r.MatchString(o)
			})),
			reqWithStore("GET"),
			corstest.TestAllowedOriginFunc,
		},
		{
			newSrv(mwcors.WithAllowedOrigins(scope.Website, 2, "http://foobar.com"), mwcors.WithAllowedMethods(scope.Website, 2, "PUT", "DELETE")),
			reqWithStore("OPTIONS"),
			corstest.TestAllowedMethod,
		},
		{
			newSrv(mwcors.WithAllowedMethods(scope.Website, 2, "PUT", "DELETE"), mwcors.WithAllowedOrigins(scope.Website, 2, "http://foobar.com"), mwcors.WithOptionsPassthrough(scope.Website, 2, true)),
			reqWithStore("OPTIONS"),
			corstest.TestAllowedMethodPassthrough,
		},
		{
			newSrv(mwcors.WithAllowedOrigins(scope.Website, 2, "http://foobar.com"), mwcors.WithAllowedHeaders(scope.Website, 2, "X-Header-1", "x-header-2")),
			reqWithStore("OPTIONS"),
			corstest.TestAllowedHeader,
		},
		{
			newSrv(mwcors.WithAllowedOrigins(scope.Website, 2, "http://foobar.com"), mwcors.WithExposedHeaders(scope.Website, 2, "X-Header-1", "x-header-2")),
			reqWithStore("GET"),
			corstest.TestExposedHeader,
		},
		{
			newSrv(mwcors.WithAllowedOrigins(scope.Website, 2, "http://foobar.com"), mwcors.WithAllowCredentials(scope.Website, 2, true)),
			reqWithStore("OPTIONS"),
			corstest.TestAllowedCredentials,
		},
		{
			newSrv(mwcors.WithMaxAge(scope.Website, 2, time.Second*30), mwcors.WithAllowedOrigins(scope.Website, 2, "http://foobar.com")),
			reqWithStore("OPTIONS"),
			corstest.TestMaxAge,
		},
	}
	for _, test := range tests {
		// for debugging comment this out to see the index which fails
		// t.Logf("Running Index %d Tester %q", i, runtime.FuncForPC(reflect.ValueOf(test.tester).Pointer()).Name())
		test.tester(t, test.srv, test.req)
	}
}

func TestMatchAllOrigin(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "*"),
	)
	req := reqWithStore("GET")
	corstest.TestMatchAllOrigin(t, s, req)
}

func TestAllowedOrigin(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
	)
	req := reqWithStore("GET")
	corstest.TestAllowedOrigin(t, s, req)
}

func TestWildcardOrigin(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://*.bar.com"),
	)
	req := reqWithStore("GET")
	corstest.TestWildcardOrigin(t, s, req)
}

func TestDisallowedOrigin(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
	)
	req := reqWithStore("GET")
	corstest.TestDisallowedOrigin(t, s, req)
}

func TestDisallowedWildcardOrigin(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://*.bar.com"),
	)
	req := reqWithStore("GET")
	corstest.TestDisallowedWildcardOrigin(t, s, req)
}

func TestAllowedOriginFunc(t *testing.T) {
	r, _ := regexp.Compile("^http://foo")
	s := mwcors.MustNew(
		mwcors.WithAllowOriginFunc(scope.Default, 0, func(o string) bool {
			return r.MatchString(o)
		}),
	)
	req := reqWithStore("GET")
	corstest.TestAllowedOriginFunc(t, s, req)
}

func TestAllowedMethod(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
		mwcors.WithAllowedMethods(scope.Default, 0, "PUT", "DELETE"),
	)
	req := reqWithStore("OPTIONS")
	corstest.TestAllowedMethod(t, s, req)
}

func TestAllowedMethodPassthrough(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
		mwcors.WithAllowedMethods(scope.Default, 0, "PUT", "DELETE"),
		mwcors.WithOptionsPassthrough(scope.Default, 0, true),
	)
	req := reqWithStore("OPTIONS")
	corstest.TestAllowedMethodPassthrough(t, s, req)
}

func TestDisallowedMethod(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
		mwcors.WithAllowedMethods(scope.Default, 0, "PUT", "DELETE"),
	)
	req := reqWithStore("OPTIONS")
	corstest.TestDisallowedMethod(t, s, req)
}

func TestAllowedHeader(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
		mwcors.WithAllowedHeaders(scope.Default, 0, "X-Header-1", "x-header-2"),
	)
	req := reqWithStore("OPTIONS")
	corstest.TestAllowedHeader(t, s, req)
}

func TestAllowedWildcardHeader(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
		mwcors.WithAllowedHeaders(scope.Default, 0, "*"),
	)
	req := reqWithStore("OPTIONS")
	corstest.TestAllowedWildcardHeader(t, s, req)
}

func TestDisallowedHeader(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
		mwcors.WithAllowedHeaders(scope.Default, 0, "X-Header-1", "x-header-2"),
	)
	req := reqWithStore("OPTIONS")
	corstest.TestDisallowedHeader(t, s, req)
}

func TestOriginHeader(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
	)
	req := reqWithStore("OPTIONS")
	corstest.TestOriginHeader(t, s, req)
}

func TestExposedHeader(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
		mwcors.WithExposedHeaders(scope.Default, 0, "X-Header-1", "x-header-2"),
	)

	req := reqWithStore("GET")
	corstest.TestExposedHeader(t, s, req)
}

func TestExposedHeader_MultiScope(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
		mwcors.WithExposedHeaders(scope.Default, 0, "X-Header-1", "x-header-2"),
		mwcors.WithAllowCredentials(scope.Website, 1, true),
	)

	reqDefault, _ := http.NewRequest("GET", "http://corestore.io/reqDefault", nil)
	reqDefault = reqDefault.WithContext(
		store.WithContextRequestedStore(reqDefault.Context(), storemock.MustNewStoreAU(cfgmock.NewService())),
	)
	corstest.TestExposedHeader(t, s, reqDefault)

	eur := storemock.NewEurozzyService(scope.Option{Website: scope.MockID(1)}, store.WithStorageConfig(cfgmock.NewService()))
	atStore, atErr := eur.Store(scope.MockID(2)) // ID = 2 store Austria
	reqWebsite, _ := http.NewRequest("OPTIONS", "http://corestore.io/reqWebsite", nil)
	reqWebsite = reqWebsite.WithContext(
		store.WithContextRequestedStore(reqWebsite.Context(), atStore, atErr),
	)
	corstest.TestAllowedCredentials(t, s, reqWebsite)
}

func TestAllowedCredentials(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
		mwcors.WithAllowCredentials(scope.Default, 0, true),
	)

	req := reqWithStore("OPTIONS")
	corstest.TestAllowedCredentials(t, s, req)
}

func TestMaxAge(t *testing.T) {
	s := mwcors.MustNew(
		mwcors.WithAllowedOrigins(scope.Default, 0, "http://foobar.com"),
		mwcors.WithMaxAge(scope.Default, 0, time.Second*30),
	)

	req := reqWithStore("OPTIONS")
	corstest.TestMaxAge(t, s, req)
}