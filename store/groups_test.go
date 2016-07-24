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

package store_test

import (
	"testing"

	"github.com/corestoreio/csfw/config/cfgmock"
	"github.com/corestoreio/csfw/storage/dbr"
	"github.com/corestoreio/csfw/store"
	"github.com/stretchr/testify/assert"
)

func TestGroupSlice_Map_Each(t *testing.T) {
	gs := store.GroupSlice{
		store.MustNewGroup(
			cfgmock.NewService(),
			&store.TableGroup{GroupID: 1, WebsiteID: 1, Name: "DACH Group", RootCategoryID: 2, DefaultStoreID: 2},
			&store.TableWebsite{WebsiteID: 1, Code: dbr.NewNullString("euro"), Name: dbr.NewNullString("Europe"), SortOrder: 0, DefaultGroupID: 1, IsDefault: dbr.NewNullBool(true)},
			store.TableStoreSlice{
				&store.TableStore{StoreID: 1, Code: dbr.NewNullString("de"), WebsiteID: 1, GroupID: 1, Name: "Germany", SortOrder: 10, IsActive: true},
			},
		),
		store.MustNewGroup(
			cfgmock.NewService(),
			&store.TableGroup{GroupID: 2, WebsiteID: 2, Name: "DACH2 Group", RootCategoryID: 2, DefaultStoreID: 2},
			&store.TableWebsite{WebsiteID: 2, Code: dbr.NewNullString("euro2"), Name: dbr.NewNullString("Europe"), SortOrder: 0, DefaultGroupID: 2, IsDefault: dbr.NewNullBool(true)},
			store.TableStoreSlice{
				&store.TableStore{StoreID: 2, Code: dbr.NewNullString("de2"), WebsiteID: 2, GroupID: 1, Name: "Germany", SortOrder: 10, IsActive: true},
			},
		),
	}

	gs.
		Map(func(g *store.Group) {
			g.Data.GroupID = 4
			g.Website.Data.Name.String = "Gopher"
		}).
		Each(func(g store.Group) {
			assert.Exactly(t, "Gopher", g.Website.Name())
		})
	assert.Exactly(t, []int64{4, 4}, gs.IDs())
}
