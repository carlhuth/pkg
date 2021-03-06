package binlogsync

import (
	"context"

	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	"github.com/corestoreio/pkg/sql/ddl"
	"golang.org/x/sync/errgroup"
)

// TODO(CyS) investigate what would happen in case of transaction? should all
// the events be gathered together once a transaction starts? because on
// RollBack all events must be invalidated or better RowsEventHandler should not
// be called at all.

// RowsEventHandler calls your code when an event gets dispatched.
type RowsEventHandler interface {
	// Do function handles a RowsEvent bound to a specific database. If it
	// returns an error behaviour of "Interrupted", the canal type will stop the
	// syncer. Binlog has three update event version, v0, v1 and v2. For v1 and
	// v2, the rows number must be even. Two rows for one event, format is
	// [before update row, after update row] for update v0, only one row for a
	// event, and we don't support this version yet. The Do function will run in
	// its own Goroutine. The provided argument `t` of type ddl.Table must only
	// be used for reading, changing `t` causes race conditions.
	Do(ctx context.Context, action string, t *ddl.Table, rows [][]interface{}) error
	// Complete runs before a binlog rotation event happens. Same error rules
	// apply here like for function Do(). The Complete function will run in its
	// own Goroutine.
	Complete(context.Context) error
	// String returns the name of the handler
	String() string
}

// RegisterRowsEventHandler adds a new event handler to the internal list. If a
// table name gets provided the event handler is bound to that exact table name,
// if the table has not been excluded via the global regexes. An empty tableName
// calls the event handler for all tables.
func (c *Canal) RegisterRowsEventHandler(tableName string, h ...RowsEventHandler) {
	c.rsMu.Lock()
	defer c.rsMu.Unlock()

	if c.rsHandlers == nil {
		c.rsHandlers = make(map[string][]RowsEventHandler)
	}
	hs := c.rsHandlers[tableName]
	c.rsHandlers[tableName] = append(hs, h...)
}

func (c *Canal) processRowsEventHandler(ctx context.Context, action string, table *ddl.Table, rows [][]interface{}) error {
	c.rsMu.RLock()
	defer c.rsMu.RUnlock()

	erg, ctx := errgroup.WithContext(ctx)

	errGoFn := func(h RowsEventHandler) func() error {
		return func() error {
			if err := h.Do(ctx, action, table, rows); err != nil {
				isInterr := errors.Is(err, errors.Interrupted)
				c.opts.Log.Info("binlogsync.Canal.processRowsEventHandler.Go.Do.error", log.Err(err), log.Stringer("handler_name", h),
					log.Bool("is_interrupted", isInterr),
					log.String("action", action), log.String("schema", c.dsn.DBName), log.String("table", table.Name))
				if isInterr {
					return errors.WithStack(err)
				}
			}
			return nil
		}
	}
	if hs, ok := c.rsHandlers[table.Name]; ok && table.Name != "" {
		for _, h := range hs {
			erg.Go(errGoFn(h))
		}
	}

	for _, h := range c.rsHandlers[""] {
		erg.Go(errGoFn(h))
	}
	return errors.WithStack(erg.Wait())
}

func (c *Canal) flushEventHandlers(ctx context.Context) error {
	defer log.WhenDone(c.opts.Log).Info("binlogsync.Canal.flushEventHandlers")
	c.rsMu.RLock()
	defer c.rsMu.RUnlock()

	erg, ctx := errgroup.WithContext(ctx)

	for tblName, hs := range c.rsHandlers {
		for _, h := range hs {
			h := h
			erg.Go(func() error {
				if err := h.Complete(ctx); err != nil {
					isInterr := errors.Is(err, errors.Interrupted)
					c.opts.Log.Info("binlogsync.Canal.flushEventHandlers.Go.Complete.error",
						log.Err(err), log.Bool("is_interrupted", isInterr), log.Stringer("handler_name", h), log.String("table_name", tblName))
					if isInterr {
						return errors.WithStack(err)
					}
				}
				return nil
			})
		}
	}
	return errors.Wrap(erg.Wait(), "[binlogsync] flushEventHandlers errgroup Wait")
}
