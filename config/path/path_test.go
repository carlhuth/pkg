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

package path_test

import (
	"errors"
	"hash/fnv"
	"strconv"
	"testing"

	"github.com/corestoreio/csfw/config/path"
	"github.com/corestoreio/csfw/storage/text"
	"github.com/corestoreio/csfw/store/scope"
	"github.com/stretchr/testify/assert"
)

func TestNewByParts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		parts   []string
		want    string
		wantErr error
	}{
		{[]string{"aa/bb/cc"}, "aa/bb/cc", nil},
		{[]string{"aa/bb", "cc"}, "aa/bb/cc", nil},
		{[]string{"aa", "bb", "cc"}, "aa/bb/cc", nil},
		{[]string{"aa", "bb", "c"}, "aa/bb/cc", path.ErrIncorrectPath},
		{nil, "", path.ErrRouteEmpty},
		{[]string{""}, "", path.ErrRouteEmpty},
	}
	for i, test := range tests {
		haveP, haveErr := path.NewByParts(test.parts...)
		if test.wantErr != nil {
			assert.Nil(t, haveP.Route.Chars, "Index %d", i)
			assert.EqualError(t, haveErr, test.wantErr.Error(), "Index %d", i)
			continue
		}
		l, err := haveP.Level(-1)
		assert.NoError(t, err, "Index %d", i)
		assert.Exactly(t, test.want, l.String(), "Index %d", i)
	}
}

func TestMustNewByPartsPanic(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			assert.EqualError(t, r.(error), path.ErrRouteInvalidBytes.Error())
		} else {
			t.Fatal("Expecting a panic")
		}
	}()
	_ = path.MustNewByParts("a/\x80/c")
}

func TestMustNewByPartsNoPanic(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			assert.NotNil(t, r, "Did not expect a panic")
		} else {
			assert.Nil(t, r, "Why is here a panic")
		}
	}()
	p := path.MustNewByParts("aa", "bb", "cc")
	assert.Exactly(t, p.String(), "default/0/aa/bb/cc")
}

var benchmarkNewByParts path.Path

// BenchmarkNewByParts-4	 5000000	       297 ns/op	      48 B/op	       1 allocs/op
func BenchmarkNewByParts(b *testing.B) {
	want := path.NewRoute("general/single_store_mode/enabled")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkNewByParts, err = path.NewByParts("general", "single_store_mode", "enabled")
		if err != nil {
			b.Error(err)
		}
	}
	if benchmarkNewByParts.Route.Equal(want) == false {
		b.Errorf("Want: %s; Have, %s", want, benchmarkNewByParts.Route)
	}
}

func TestPathNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		route      path.Route
		s          scope.Scope
		id         int64
		wantFQ     path.Route
		wantNewErr error
	}{
		{path.NewRoute("ab/b\x80/cd"), scope.WebsiteID, 3, path.NewRoute("websites/3/ab/ba/cd"), path.ErrRouteInvalidBytes},
		{path.NewRoute("ab/ba/cd"), scope.WebsiteID, 3, path.NewRoute("websites/3/ab/ba/cd"), nil},
		{path.NewRoute("ad/ba/ca/sd"), scope.WebsiteID, 3, path.NewRoute("websites/3/ad/ba/ca/sd"), nil},
		{path.NewRoute("as/sb"), scope.WebsiteID, 3, path.NewRoute("websites/3/a/b/c/d"), path.ErrIncorrectPath},
		{path.NewRoute("aa/bb/cc"), scope.GroupID, 3, path.NewRoute("default/0/aa/bb/cc"), nil},
		{path.NewRoute("aa/bb/cc"), scope.StoreID, 3, path.NewRoute("stores/3/aa/bb/cc"), nil},
	}
	for i, test := range tests {
		haveP, haveErr := path.New(test.route)
		haveP = haveP.Bind(test.s, test.id)
		if test.wantNewErr != nil {
			assert.EqualError(t, haveErr, test.wantNewErr.Error(), "Index %d", i)
			continue
		}
		fq, fqErr := haveP.FQ()
		assert.NoError(t, fqErr, "Index %d", i)
		assert.Exactly(t, test.wantFQ, fq, "Index %d", i)
	}
}

func TestFQ(t *testing.T) {
	t.Parallel()
	tests := []struct {
		str     scope.StrScope
		id      int64
		route   path.Route
		want    string
		wantErr error
	}{
		{scope.StrDefault, 0, path.Route{}, "", path.ErrRouteEmpty},
		{scope.StrDefault, 0, path.NewRoute(""), "", path.ErrRouteEmpty},
		{scope.StrDefault, 0, path.NewRoute("system/dev/debug"), scope.StrDefault.String() + "/0/system/dev/debug", nil},
		{scope.StrDefault, 44, path.NewRoute("system/dev/debug"), scope.StrDefault.String() + "/0/system/dev/debug", nil},
		{scope.StrWebsites, 0, path.NewRoute("system/dev/debug"), scope.StrWebsites.String() + "/0/system/dev/debug", nil},
		{scope.StrWebsites, 343, path.NewRoute("system/dev/debug"), scope.StrWebsites.String() + "/343/system/dev/debug", nil},
		{scope.StrScope("hello"), 0, path.NewRoute("system/dev/debug"), scope.StrDefault.String() + "/0/system/dev/debug", nil},
		{scope.StrScope("hello"), 343, path.NewRoute("system/dev/debug"), scope.StrDefault.String() + "/0/system/dev/debug", nil},
	}
	for i, test := range tests {
		p, pErr := path.New(test.route)
		p = p.BindStr(test.str, test.id)
		have, haveErr := p.FQ()
		if test.wantErr != nil {
			assert.Empty(t, have.Chars, "Index %d", i)
			if pErr != nil {
				assert.EqualError(t, pErr, test.wantErr.Error(), "Index %d", i)
				continue
			}
			assert.EqualError(t, haveErr, test.wantErr.Error(), "Index %d", i)
			continue
		}
		assert.NoError(t, haveErr, "Index %d", i)
		assert.Exactly(t, test.want, have.String(), "Index %d", i)
	}

	r := path.NewRoute("catalog/frontend/list_allow_all")
	assert.Exactly(t, "stores/7475/catalog/frontend/list_allow_all", path.MustNew(r).BindStr(scope.StrStores, 7475).String())
	p := path.MustNew(r).BindStr(scope.StrStores, 5)
	assert.Exactly(t, "stores/5/catalog/frontend/list_allow_all", p.String())
	assert.Exactly(t, "path.Path{ Route:path.NewRoute(`catalog/frontend/list_allow_all`), Scope: 4, ID: 5 }", p.GoString())
}

func TestShouldNotPanicBecauseOfIncorrectStrScope(t *testing.T) {
	t.Parallel()
	assert.Exactly(t, "stores/345/xxxxx/yyyyy/zzzzz", path.MustNew(path.NewRoute("xxxxx/yyyyy/zzzzz")).BindStr(scope.StrStores, 345).String())
	defer func() {
		if r := recover(); r != nil {
			t.Fatal("Did not expect a panic")
		}
	}()
	_ = path.MustNew(path.NewRoute("xxxxx/yyyyy/zzzzz")).BindStr(scope.StrScope("invalid"), 345)
}

func TestShouldPanicIncorrectPath(t *testing.T) {
	t.Parallel()
	assert.Exactly(t, "default/0/xxxxx/yyyyy/zzzzz", path.MustNew(path.NewRoute("xxxxx/yyyyy/zzzzz")).BindStr(scope.StrDefault, 345).String())
	defer func() {
		if r := recover(); r != nil {
			assert.EqualError(t, r.(error), path.ErrIncorrectPath.Error())
		} else {
			t.Fatal("Expecting a panic")
		}
	}()
	assert.Exactly(t, "websites/345/xxxxx/yyyyy", path.MustNew(path.NewRoute("xxxxx/yyyyy")).BindStr(scope.StrWebsites, 345).String())
}

var benchmarkStrScopeFQPath path.Route

func benchmarkFQ(scopeID int64, b *testing.B) {
	want := path.NewRoute(scope.StrWebsites.String() + "/" + strconv.FormatInt(scopeID, 10) + "/system/dev/debug")
	p := path.NewRoute("system/dev/debug")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkStrScopeFQPath, err = path.MustNew(p).BindStr(scope.StrWebsites, scopeID).FQ()
		if err != nil {
			b.Error(err)
		}
	}
	if benchmarkStrScopeFQPath.Equal(want) == false {
		b.Errorf("Want: %s; Have, %s", want, benchmarkStrScopeFQPath)
	}
}

// BenchmarkFQ-4     	 3000000	       401 ns/op	     112 B/op	       1 allocs/op
func BenchmarkFQ(b *testing.B) {
	benchmarkFQ(11, b)
}

func TestSplitFQ(t *testing.T) {
	t.Parallel()
	tests := []struct {
		have        string
		wantScope   string
		wantScopeID int64
		wantPath    string
		wantErr     error
	}{
		{"groups/1/catalog/frontend/list_allow_all", "default", 0, "", scope.ErrUnsupportedScope},
		{"stores/7475/catalog/frontend/list_allow_all", scope.StrStores.String(), 7475, "catalog/frontend/list_allow_all", nil},
		{"stores/4/system/full_page_cache/varnish/backend_port", scope.StrStores.String(), 4, "system/full_page_cache/varnish/backend_port", nil},
		{"websites/1/catalog/frontend/list_allow_all", scope.StrWebsites.String(), 1, "catalog/frontend/list_allow_all", nil},
		{"default/0/catalog/frontend/list_allow_all", scope.StrDefault.String(), 0, "catalog/frontend/list_allow_all", nil},
		{"default//catalog/frontend/list_allow_all", scope.StrDefault.String(), 0, "catalog/frontend/list_allow_all", errors.New("strconv.ParseInt: parsing \"\\uf8ff\": invalid syntax")},
		{"stores/123/catalog/index", "default", 0, "", errors.New("Incorrect fully qualified path: \"stores/123/catalog/index\"")},
	}
	for _, test := range tests {
		havePath, haveErr := path.SplitFQ(test.have)

		if test.wantErr != nil {
			assert.EqualError(t, haveErr, test.wantErr.Error(), "Test %v", test)
		} else {
			assert.NoError(t, haveErr, "Test %v", test)
		}
		assert.Exactly(t, test.wantScope, havePath.StrScope(), "Test %v", test)
		assert.Exactly(t, test.wantScopeID, havePath.ID, "Test %v", test)
		l, _ := havePath.Level(-1)
		assert.Exactly(t, test.wantPath, l.String(), "Test %v", test)
	}
}

var benchmarkReverseFQPath path.Path

// BenchmarkSplitFQ-4  	10000000	       199 ns/op	      32 B/op	       1 allocs/op
func BenchmarkSplitFQ(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkReverseFQPath, err = path.SplitFQ("stores/7475/catalog/frontend/list_allow_all")
		if err != nil {
			b.Error(err)
		}
	}
	l, _ := benchmarkReverseFQPath.Level(-1)
	if l.String() != "catalog/frontend/list_allow_all" {
		b.Error("catalog/frontend/list_allow_all not found in Level()")
	}
}

func TestPathIsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s    scope.Scope
		id   int64
		have path.Route
		want error
	}{
		{scope.DefaultID, 0, path.NewRoute("//"), path.ErrIncorrectPath},
		{scope.DefaultID, 0, path.NewRoute("general/store_information/city"), nil},
		{scope.DefaultID, 33, path.NewRoute("general/store_information/city"), nil},
		{scope.WebsiteID, 33, path.NewRoute("system/full_page_cache/varnish/backend_port"), nil},
		{scope.DefaultID, 0, path.NewRoute(""), path.ErrRouteEmpty},
		{scope.DefaultID, 0, path.NewRoute("general/store_information"), path.ErrIncorrectPath},
		////{path.NewRoute(path.MustNew("system/dev/debug").Bind(scope.WebsiteID, 22).String()), path.ErrIncorrectPath},
		{scope.DefaultID, 0, path.NewRoute("groups/33/general/store_information/street"), nil},
		{scope.DefaultID, 0, path.NewRoute("groups/33"), path.ErrIncorrectPath},
		{scope.DefaultID, 0, path.NewRoute("system/dEv/inv˚lid"), errors.New("This character \"˚\" is not allowed in Route system/dEv/inv˚lid")},
		{scope.DefaultID, 0, path.NewRoute("system/dEv/inv'lid"), errors.New("This character \"'\" is not allowed in Route system/dEv/inv'lid")},
		{scope.DefaultID, 0, path.NewRoute("syst3m/dEv/invalid"), nil},
		{scope.DefaultID, 0, path.Route{}, path.ErrRouteEmpty},
	}
	for i, test := range tests {
		p := path.Path{
			Scope: test.s,
			ID:    test.id,
			Route: test.have,
		}
		haveErr := p.IsValid()
		if test.want != nil {
			assert.EqualError(t, haveErr, test.want.Error(), "Index %d", i)
		} else {
			assert.NoError(t, haveErr, "Index %d", i)
		}
	}
}

func TestPathRouteIsValid(t *testing.T) {
	t.Parallel()
	p := path.Path{
		Scope: scope.StoreID,
		ID:    2,
		Route: path.NewRoute(`general/store_information`),
	}
	assert.EqualError(t, p.IsValid(), path.ErrIncorrectPath.Error())

	p = path.Path{
		Scope:           scope.StoreID,
		ID:              2,
		Route:           path.NewRoute(`general/store_information`),
		RouteLevelValid: true,
	}
	assert.NoError(t, p.IsValid())
}

func TestPathHash(t *testing.T) {
	t.Parallel()
	tests := []struct {
		have      path.Route
		level     int
		wantHash  uint32
		wantErr   error
		wantLevel string
	}{
		{path.NewRoute("general/single_\x80store_mode/enabled"), 0, 0, path.ErrRouteInvalidBytes, ""},
		{path.NewRoute("general/single_store_mode/enabled"), 0, 2166136261, nil, ""},
		{path.NewRoute("general/single_store_mode/enabled"), 1, 616112491, nil, "general"},
		{path.NewRoute("general/single_store_mode/enabled"), 2, 2274889228, nil, "general/single_store_mode"},
		{path.NewRoute("general/single_store_mode/enabled"), 3, 1644245266, nil, "general/single_store_mode/enabled"},
		{path.NewRoute("general/single_store_mode/enabled"), -1, 1644245266, nil, "general/single_store_mode/enabled"},
		{path.NewRoute("general/single_store_mode/enabled"), 5, 1644245266, nil, "general/single_store_mode/enabled"},
		{path.NewRoute("general/single_store_mode/enabled"), 4, 1644245266, nil, "general/single_store_mode/enabled"},
	}
	for i, test := range tests {
		p := path.Path{
			Route: test.have,
		}

		hv, err := p.Hash(test.level)
		if test.wantErr != nil {
			assert.EqualError(t, err, test.wantErr.Error(), "Index %d", i)
			assert.Empty(t, hv, "Index %d", i)
			continue
		}
		assert.NoError(t, err, "Index %d", i)

		check := fnv.New32a()
		_, cErr := check.Write([]byte(test.wantLevel))
		assert.NoError(t, cErr)
		assert.Exactly(t, check.Sum32(), hv, "Index %d", i)

		l, err := p.Level(test.level)
		assert.Exactly(t, test.wantLevel, l.String(), "Index %d", i)
		assert.Exactly(t, test.wantHash, hv, "Want %d Have %d Index %d", test.wantHash, hv, i)
	}
}

func TestPathPartPosition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		have     path.Route
		level    int
		wantPart string
		wantErr  error
	}{
		{path.NewRoute("general/single_\x80store_mode/enabled"), 0, "", path.ErrIncorrectPosition},
		{path.NewRoute("general/single_store_mode/enabled"), 0, "", path.ErrIncorrectPosition},
		{path.NewRoute("general/single_store_mode/enabled"), 1, "general", nil},
		{path.NewRoute("general/single_store_mode/enabled"), 2, "single_store_mode", nil},
		{path.NewRoute("general/single_store_mode/enabled"), 3, "enabled", nil},
		{path.NewRoute("general/single_store_mode/enabled"), -1, "", path.ErrIncorrectPosition},
		{path.NewRoute("general/single_store_mode/enabled"), 5, "", path.ErrIncorrectPosition},
		{path.NewRoute("general/single/store/website/group/mode/enabled/disabled/default"), 5, "group", nil},
		{path.NewRoute("system/full_page_cache/varnish/backend_port"), 3, "varnish", nil},
		{path.NewRoute("system/full_page_cache/varnish/backend_port"), 4, "backend_port", nil},
	}
	for i, test := range tests {
		p := path.Path{
			Route: test.have,
		}
		part, haveErr := p.Part(test.level)
		if test.wantErr != nil {
			assert.EqualError(t, haveErr, test.wantErr.Error(), "Index %d", i)
			assert.Nil(t, part.Chars, "Index %d", i)
			continue
		}
		assert.Exactly(t, test.wantPart, part.String(), "Index %d", i)
	}
}

func TestPathCloneRareUseCase(t *testing.T) {
	t.Parallel()
	rs := "aa/bb/cc"
	pOrg := path.MustNewByParts(rs)
	pOrg = pOrg.Bind(scope.StoreID, 3141)

	largerBuff := make(text.Chars, 100, 100)
	pOrg.Chars = largerBuff[:copy(largerBuff, rs)]

	pAssigned := pOrg
	pCloned := pOrg.Clone()

	assert.Exactly(t, pOrg.Scope, pCloned.Scope)
	assert.Exactly(t, pOrg.ID, pCloned.ID)
	assert.Exactly(t, pOrg.Route, pCloned.Route)

	assert.Exactly(t, pOrg.Scope, pAssigned.Scope)
	assert.Exactly(t, pOrg.ID, pAssigned.ID)
	assert.Exactly(t, pOrg.Route, pAssigned.Route)

	// we're not using Path.Append because it creates internally a new byte slice
	// this append() grows the slice without creating a new one because the cap == 100, see above.
	pOrg.Chars = append(pOrg.Chars, []byte(`/dd`)...)

	assert.Exactly(t, "stores/3141/"+rs+"/dd", pOrg.String())

	assert.Exactly(t, "stores/3141/"+rs, pAssigned.String())

	assert.NotEqual(t, pOrg, pAssigned)

	// now expand the slice
	pAssigned.Chars = pAssigned.Chars[:len(pOrg.Chars)]
	assert.Exactly(t, "stores/3141/"+rs+"/dd", pAssigned.String())
	assert.Exactly(t, pOrg, pAssigned)
	assert.Exactly(t, "stores/3141/"+rs, pCloned.String())
	assert.NotEqual(t, pOrg, pCloned)
}

func TestPathCloneAppend(t *testing.T) {
	t.Parallel()
	rs := "aa/bb/cc"
	pOrg := path.MustNewByParts(rs)
	pOrg = pOrg.Bind(scope.StoreID, 3141)

	pAssigned := pOrg
	assert.Exactly(t, pOrg, pAssigned)
	assert.NoError(t, pOrg.Append(path.NewRoute("dd")))
	assert.NotEqual(t, pOrg, pAssigned)
}
