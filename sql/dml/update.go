// Copyright 2015-present, Cyrill @ Schumacher.fm and the CoreStore contributors
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

package dml

import (
	"bytes"
	"context"

	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
)

// Update contains the clauses for an UPDATE statement
type Update struct {
	BuilderBase
	BuilderConditional

	// TODO: add UPDATE JOINS SQLStmtUpdateJoin

	// SetClausAliases only applicable in case when field QualifiedRecords has
	// been set or ExecMulti gets used. `SetClausAliases` contains the lis of
	// column names which gets passed to the ColumnMapper. If empty,
	// `SetClausAliases` collects the column names from the `SetClauses`. The
	// alias slice must have the same length as the columns slice. Despite
	// setting `SetClausAliases` the SetClauses.Columns must be provided to
	// create a valid SQL statement.
	SetClausAliases []string
	// SetClauses contains the column/argument association. For each column
	// there must be one argument.
	SetClauses Conditions
	// Listeners allows to dispatch certain functions in different
	// situations.
	Listeners ListenersUpdate
}

// NewUpdate creates a new Update object.
func NewUpdate(table string) *Update {
	return &Update{
		BuilderBase: BuilderBase{
			Table: MakeIdentifier(table),
		},
	}
}

func newUpdate(db QueryExecPreparer, idFn uniqueIDFn, l log.Logger, table string) *Update {
	id := idFn()
	if l != nil {
		l = l.With(log.String("update_id", id), log.String("table", table))
	}
	return &Update{
		BuilderBase: BuilderBase{
			builderCommon: builderCommon{
				id:  id,
				Log: l,
				DB:  db,
			},
			Table: MakeIdentifier(table),
		},
	}
}

// Update creates a new Update for the given table with a random connection from
// the pool.
func (c *ConnPool) Update(table string) *Update {
	return newUpdate(c.DB, c.makeUniqueID, c.Log, table)
}

// Update creates a new Update for the given table bound to a single connection.
func (c *Conn) Update(table string) *Update {
	return newUpdate(c.DB, c.makeUniqueID, c.Log, table)
}

// Update creates a new Update for the given table bound to a transaction.
func (tx *Tx) Update(table string) *Update {
	return newUpdate(tx.DB, tx.makeUniqueID, tx.Log, table)
}

// Alias sets an alias for the table name.
func (b *Update) Alias(alias string) *Update {
	b.Table.Aliased = alias
	return b
}

// WithDB sets the database query object.
func (b *Update) WithDB(db QueryExecPreparer) *Update {
	b.DB = db
	return b
}

// Unsafe see BuilderBase.IsUnsafe which weakens security when building the SQL
// string. This function must be called before calling any other function.
func (b *Update) Unsafe() *Update {
	b.IsUnsafe = true
	return b
}

// Set appends a column/value pair for the statement.
func (b *Update) Set(c ...*Condition) *Update {
	b.SetClauses = append(b.SetClauses, c...)
	return b
}

// AddColumns adds columns which values gets later derived from a ColumnMapper.
// Those columns will get passed to the ColumnMapper implementation.
func (b *Update) AddColumns(columnNames ...string) *Update {
	for _, col := range columnNames {
		b.SetClauses = append(b.SetClauses, Column(col))
	}
	return b
}

// Where appends a WHERE clause to the statement
func (b *Update) Where(wf ...*Condition) *Update {
	b.Wheres = append(b.Wheres, wf...)
	return b
}

// OrderBy appends columns to the ORDER BY statement for ascending sorting. A
// column gets always quoted if it is a valid identifier otherwise it will be
// treated as an expression. When you use ORDER BY or GROUP BY to sort a column
// in a UPDATE, the server sorts values using only the initial number of bytes
// indicated by the max_sort_length system variable.
func (b *Update) OrderBy(columns ...string) *Update {
	b.OrderBys = b.OrderBys.AppendColumns(b.IsUnsafe, columns...)
	return b
}

// OrderByDesc appends columns to the ORDER BY statement for descending sorting.
// A column gets always quoted if it is a valid identifier otherwise it will be
// treated as an expression. When you use ORDER BY or GROUP BY to sort a column
// in a UPDATE, the server sorts values using only the initial number of bytes
// indicated by the max_sort_length system variable.
func (b *Update) OrderByDesc(columns ...string) *Update {
	b.OrderBys = b.OrderBys.AppendColumns(b.IsUnsafe, columns...).applySort(len(columns), sortDescending)
	return b
}

// Limit sets a limit for the statement; overrides any existing LIMIT
func (b *Update) Limit(limit uint64) *Update {
	b.LimitCount = limit
	b.LimitValid = true
	return b
}

// WithArgs builds the SQL string and sets the optional interfaced arguments for
// the later execution. It copies the underlying connection and structs.
func (b *Update) WithArgs(args ...interface{}) *Arguments {
	b.source = dmlSourceUpdate
	return b.withArgs(b, args...)
}

// ToSQL converts the select statement into a string and returns its arguments.
func (b *Update) ToSQL() (string, []interface{}, error) {
	b.source = dmlSourceUpdate
	rawSQL, err := b.buildToSQL(b)
	if err != nil {
		return "", nil, errors.WithStack(err)
	}
	return string(rawSQL), nil, nil
}

func (b *Update) writeBuildCache(sql []byte) {
	b.BuilderConditional = BuilderConditional{}
	b.SetClausAliases = nil
	b.SetClauses = nil
	b.cachedSQL = sql
}

func (b *Update) readBuildCache() (sql []byte) {
	return b.cachedSQL
}

// DisableBuildCache if enabled it does not cache the SQL string as a final
// rendered byte slice. Allows you to rebuild the query with different
// statements.
func (b *Update) DisableBuildCache() *Update {
	b.IsBuildCacheDisabled = true
	return b
}

// ToSQL serialized the Update to a SQL string
// It returns the string with placeholders and a slice of query arguments
func (b *Update) toSQL(buf *bytes.Buffer, placeHolders []string) ([]string, error) {
	b.defaultQualifier = b.Table.qualifier()

	if err := b.Listeners.dispatch(OnBeforeToSQL, b); err != nil {
		return nil, errors.WithStack(err)
	}

	if b.RawFullSQL != "" {
		buf.WriteString(b.RawFullSQL)
		return placeHolders, nil
	}

	if len(b.Table.Name) == 0 {
		return nil, errors.Empty.Newf("[dml] Update: Table at empty")
	}
	if len(b.SetClauses) == 0 {
		return nil, errors.Empty.Newf("[dml] Update: No columns specified")
	}

	buf.WriteString("UPDATE ")
	writeStmtID(buf, b.id)
	_, _ = b.Table.writeQuoted(buf, nil)
	buf.WriteString(" SET ")

	placeHolders, err := b.SetClauses.writeSetClauses(buf, placeHolders)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Write WHERE clause if we have any fragments
	placeHolders, err = b.Wheres.write(buf, 'w', placeHolders)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sqlWriteOrderBy(buf, b.OrderBys, false)
	sqlWriteLimitOffset(buf, b.LimitValid, b.LimitCount, false, 0)
	return placeHolders, nil
}

func (b *Update) validate() error {
	if len(b.cachedSQL) > 1 { // already validated
		return nil
	}
	if len(b.SetClauses) == 0 {
		return errors.Empty.Newf("[dml] Update: Columns are empty")
	}
	if len(b.SetClausAliases) > 0 && len(b.SetClausAliases) != len(b.SetClauses) {
		return errors.Mismatch.Newf("[dml] Update: ColumnAliases slice and Columns slice must have the same length")
	}
	return nil
}

// Prepare executes the statement represented by the Update to create a prepared
// statement. It returns a custom statement type or an error if there was one.
// Provided arguments or records in the Update are getting ignored. The provided
// context is used for the preparation of the statement, not for the execution
// of the statement. The returned Stmter is not safe for concurrent use, despite
// the underlying *sql.Stmt is.
func (b *Update) Prepare(ctx context.Context) (*Stmt, error) {
	if err := b.validate(); err != nil {
		return nil, errors.WithStack(err)
	}
	return b.prepare(ctx, b.DB, b, dmlSourceUpdate)
}
