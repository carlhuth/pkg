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

package path

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/corestoreio/csfw/store/scope"
	"github.com/corestoreio/csfw/util/bufferpool"
)

// PS path separator used in the database table core_config_data and in config.Service
const PS = "/"

const strDefaultID = "0"

// FQ returns the fully qualified path. scopeID is an int string. Paths is
// either one path (system/smtp/host) including path separators or three
// parts ("system", "smtp", "host").
func FQ(s scope.StrScope, scopeID string, paths ...string) (string, error) {
	if false == scope.Valid(s.String()) {
		return "", scope.ErrUnsupportedScope
	}
	if s == scope.StrDefault && scopeID != strDefaultID {
		scopeID = strDefaultID // default scope is always 0
	}
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	buf.WriteString(string(s))
	buf.WriteString(PS)
	buf.WriteString(scopeID)
	buf.WriteString(PS)
	for i, p := range paths {
		buf.WriteString(p)
		if i < (len(paths) - 1) {
			buf.WriteString(PS)
		}
	}
	return buf.String(), nil
}

// MustFQ same as FQ but panics on error.
func MustFQ(s scope.StrScope, scopeID string, paths ...string) string {
	p, err := FQ(s, scopeID, paths...)
	if err != nil {
		panic(err)
	}
	return p
}

// this "cache" should cover ~80% of all store setups
var int64Cache = [...]string{
	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20",
}
var int64CacheLen = int64(len(int64Cache))

// FQInt64 same as FQPath() but for int64 scope IDs.
func FQInt64(s scope.StrScope, scopeID int64, paths ...string) (string, error) {
	idStr := "0"
	if scopeID > 0 {
		if scopeID <= int64CacheLen {
			idStr = int64Cache[scopeID]
		} else {
			idStr = strconv.FormatInt(scopeID, 10)
		}
	}
	return FQ(s, idStr, paths...)
}

// MustFQInt64 same as FQInt64 but panics on error.
func MustFQInt64(s scope.StrScope, scopeID int64, paths ...string) string {
	p, err := FQInt64(s, scopeID, paths...)
	if err != nil {
		panic(err)
	}
	return p
}

// plit splits a configuration path by the path separator PS.
func Split(path string) []string {
	return strings.Split(path, PS)
}

// Join joins configuration path parts by the path separator PS.
func Join(path ...string) string {
	return strings.Join(path, PS)
}

// SplitFQPath takes a fully qualified path and splits it into its parts.
// 	Input: stores/5/catalog/frontend/list_allow_all
//	=>
//		scope: 		stores
//		scopeID: 	5
//		path: 		catalog/frontend/list_allow_all
// Zero allocations to memory. Err may contain an ErrUnsupportedScope or
// failed to parse a string into an int64 or invalid fqPath.
func SplitFQ(fqPath string) (scopeStr string, scopeID int64, path string, err error) {
	if strings.Count(fqPath, PS) < 4 {
		err = fmt.Errorf("Incorrect fully qualified path: %q", fqPath)
		return
	}

	fi := strings.Index(fqPath, PS)
	scopeStr = fqPath[:fi]

	if false == scope.Valid(scopeStr) {
		err = scope.ErrUnsupportedScope
		return
	}

	fqPath = fqPath[fi+1:]

	fi = strings.Index(fqPath, PS)
	scopeID, err = strconv.ParseInt(fqPath[:fi], 10, 64)
	path = fqPath[fi+1:]
	return
}
