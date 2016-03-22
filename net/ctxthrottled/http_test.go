// Copyright (c) 2014, Martin Angers and Contributors.
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
//
// * Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
//
// * Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.
//
// * Neither the name of the author nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// Copyright 2015-2016, Cyrill @ Schumacher.fm and the CoreStore contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ctxthrottled_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/corestoreio/csfw/config"
	"github.com/corestoreio/csfw/config/cfgmock"
	"github.com/corestoreio/csfw/net/ctxhttp"
	"github.com/corestoreio/csfw/net/ctxthrottled"
	"github.com/corestoreio/csfw/store"
	"github.com/corestoreio/csfw/store/scope"
	"github.com/corestoreio/csfw/store/storemock"
	"github.com/corestoreio/csfw/store/storenet"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"gopkg.in/throttled/throttled.v2"
)

type stubLimiter struct {
}

func (sl stubLimiter) RateLimit(key string, quantity int) (bool, throttled.RateLimitResult, error) {
	switch key {
	case "limit":
		return true, throttled.RateLimitResult{-1, -1, -1, time.Minute}, nil
	case "error":
		return false, throttled.RateLimitResult{}, errors.New("stubLimiter error")
	default:
		return false, throttled.RateLimitResult{1, 2, time.Minute, -1}, nil
	}
}

func newStubLimiter(returnedErr error) ctxthrottled.RateLimiterFactory {
	return func(*ctxthrottled.PkgBackend, config.ScopedGetter) (throttled.RateLimiter, error) {
		return stubLimiter{}, returnedErr
	}
}

type pathGetter struct{}

func (pathGetter) Key(r *http.Request) string {
	return r.URL.Path
}

type httpTestCase struct {
	path    string
	code    int
	headers map[string]string
}

var finalHandler200 = ctxhttp.HandlerFunc(func(_ context.Context, w http.ResponseWriter, _ *http.Request) error {
	w.WriteHeader(200)
	return nil
})

func TestHTTPRateLimit_WithoutConfig(t *testing.T) {
	t.Parallel()
	// this test case runs without the backend configuration because
	// we're using WithScopedRateLimiter() to set a rate limiter for a specific
	// website (ID 1). In real life you must create a rate limiter for each website
	// or we can implement a configurable pass-through option which bypasses the RL. that
	// means if a rate limiter cannot be found the next HTTP handler gets called.

	limiter, err := ctxthrottled.NewHTTPRateLimit(
		ctxthrottled.WithVaryBy(pathGetter{}),
		ctxthrottled.WithScopedRateLimiter(scope.WebsiteID, 1, stubLimiter{}), // 1 = NewEurozzyService() website euro
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx := storenet.WithContextProvider(
		context.Background(),
		storemock.NewEurozzyService(scope.MustSetByCode(scope.WebsiteID, "euro")),
	)

	handler := limiter.WithRateLimit()(finalHandler200)

	runHTTPTestCases(t, ctx, handler, []httpTestCase{
		{"ok", 200, map[string]string{"X-Ratelimit-Limit": "1", "X-Ratelimit-Remaining": "2", "X-Ratelimit-Reset": "60"}},
		{"error", 500, map[string]string{}},
		{"limit", 429, map[string]string{"Retry-After": "60"}},
	})
}

func TestHTTPRateLimit_WithConfig(t *testing.T) {
	t.Parallel()

	cfgStruct, err := ctxthrottled.NewConfigStructure()
	if err != nil {
		t.Fatal(err)
	}

	limiter, err := ctxthrottled.NewHTTPRateLimit(
		ctxthrottled.WithVaryBy(pathGetter{}),
		ctxthrottled.WithBackend(cfgStruct),
	)
	if err != nil {
		t.Fatal(err)
	}

	cr := cfgmock.NewService(
		cfgmock.WithPV(cfgmock.PathValue{
			limiter.Backend.RateLimitBurst.MustFQ(scope.WebsiteID, 1):    0,
			limiter.Backend.RateLimitRequests.MustFQ(scope.WebsiteID, 1): 1,
			limiter.Backend.RateLimitDuration.MustFQ(scope.WebsiteID, 1): "i",
		}),
	)
	ctx := storenet.WithContextProvider(
		context.Background(),
		storemock.NewEurozzyService(
			scope.MustSetByCode(scope.WebsiteID, "euro"),
			store.WithStorageConfig(cr),
		),
	)

	handler := limiter.WithRateLimit()(finalHandler200)

	runHTTPTestCases(t, ctx, handler, []httpTestCase{
		{"xx", 200, map[string]string{"X-Ratelimit-Limit": "1", "X-Ratelimit-Remaining": "0", "X-Ratelimit-Reset": "60"}},
		{"xx", 429, map[string]string{"X-Ratelimit-Limit": "1", "X-Ratelimit-Remaining": "0", "X-Ratelimit-Reset": "60", "Retry-After": "60"}},
	})
}

func TestHTTPRateLimit_CustomHandlers(t *testing.T) {
	t.Parallel()
	limiter, err := ctxthrottled.NewHTTPRateLimit(
		ctxthrottled.WithVaryBy(pathGetter{}),
		ctxthrottled.WithRateLimiterFactory(newStubLimiter(nil)),
	)
	if err != nil {
		t.Fatal(err)
	}

	limiter.DeniedHandler = ctxhttp.HandlerFunc(func(_ context.Context, w http.ResponseWriter, _ *http.Request) error {
		http.Error(w, "custom limit exceeded", 400)
		return nil
	})

	ctx := storenet.WithContextProvider(
		context.Background(),
		storemock.NewEurozzyService(
			scope.MustSetByCode(scope.WebsiteID, "euro"),
		),
	)

	handler := limiter.WithRateLimit()(finalHandler200)

	runHTTPTestCases(t, ctx, handler, []httpTestCase{
		{"limit", 400, map[string]string{}},
		{"error", 500, map[string]string{}},
	})
}

func TestHTTPRateLimit_RateLimiterFactoryError(t *testing.T) {
	t.Parallel()

	testedErr := errors.New("RateLimiterFactory Error")

	limiter, err := ctxthrottled.NewHTTPRateLimit(
		ctxthrottled.WithVaryBy(pathGetter{}),
		ctxthrottled.WithRateLimiterFactory(newStubLimiter(testedErr)),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx := storenet.WithContextProvider(
		context.Background(),
		storemock.NewEurozzyService(
			scope.MustSetByCode(scope.WebsiteID, "euro"),
		),
	)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = limiter.WithRateLimit()(finalHandler200).ServeHTTPContext(ctx, nil, req)
	assert.EqualError(t, err, testedErr.Error())
}

func TestHTTPRateLimit_MissingContext(t *testing.T) {
	t.Parallel()

	limiter, err := ctxthrottled.NewHTTPRateLimit(
		ctxthrottled.WithVaryBy(pathGetter{}),
		ctxthrottled.WithRateLimiterFactory(newStubLimiter(nil)),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx := storenet.WithContextProvider(
		context.Background(),
		nil,
	)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = limiter.WithRateLimit()(finalHandler200).ServeHTTPContext(ctx, nil, req)
	assert.EqualError(t, err, storenet.ErrContextProviderNotFound.Error())
}

func runHTTPTestCases(t *testing.T, ctx context.Context, h ctxhttp.Handler, cs []httpTestCase) {
	for i, c := range cs {
		req, err := http.NewRequest("GET", c.path, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		if err := h.ServeHTTPContext(ctx, rr, req); err != nil {
			http.Error(rr, err.Error(), http.StatusInternalServerError)
		}

		if have, want := rr.Code, c.code; have != want {
			t.Errorf("Expected request %d at %s to return %d but got %d",
				i, c.path, want, have)
		}

		for name, want := range c.headers {
			if have := rr.HeaderMap.Get(name); have != want {
				t.Errorf("Expected request %d at %s to have header '%s: %s' but got '%s'",
					i, c.path, name, want, have)
			}
		}
	}
}
