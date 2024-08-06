// Code generated by SQLBoiler 4.16.2 (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/friendsofgo/errors"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"github.com/volatiletech/sqlboiler/v4/queries/qmhelper"
	"github.com/volatiletech/strmangle"
)

// Session is an object representing the database table.
type Session struct {
	ID           string      `boil:"id" json:"id" toml:"id" yaml:"id"`
	CreatedAt    time.Time   `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	UpdatedAt    time.Time   `boil:"updated_at" json:"updated_at" toml:"updated_at" yaml:"updated_at"`
	UserID       null.String `boil:"user_id" json:"user_id,omitempty" toml:"user_id" yaml:"user_id,omitempty"`
	ConnectionID null.String `boil:"connection_id" json:"connection_id,omitempty" toml:"connection_id" yaml:"connection_id,omitempty"`
	ExpiresAt    time.Time   `boil:"expires_at" json:"expires_at" toml:"expires_at" yaml:"expires_at"`
	LastUsed     time.Time   `boil:"last_used" json:"last_used" toml:"last_used" yaml:"last_used"`
	Deleted      bool        `boil:"deleted" json:"deleted" toml:"deleted" yaml:"deleted"`
	Meta         []byte      `boil:"meta" json:"meta" toml:"meta" yaml:"meta"`

	R *sessionR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L sessionL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

var SessionColumns = struct {
	ID           string
	CreatedAt    string
	UpdatedAt    string
	UserID       string
	ConnectionID string
	ExpiresAt    string
	LastUsed     string
	Deleted      string
	Meta         string
}{
	ID:           "id",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
	UserID:       "user_id",
	ConnectionID: "connection_id",
	ExpiresAt:    "expires_at",
	LastUsed:     "last_used",
	Deleted:      "deleted",
	Meta:         "meta",
}

var SessionTableColumns = struct {
	ID           string
	CreatedAt    string
	UpdatedAt    string
	UserID       string
	ConnectionID string
	ExpiresAt    string
	LastUsed     string
	Deleted      string
	Meta         string
}{
	ID:           "sessions.id",
	CreatedAt:    "sessions.created_at",
	UpdatedAt:    "sessions.updated_at",
	UserID:       "sessions.user_id",
	ConnectionID: "sessions.connection_id",
	ExpiresAt:    "sessions.expires_at",
	LastUsed:     "sessions.last_used",
	Deleted:      "sessions.deleted",
	Meta:         "sessions.meta",
}

// Generated where

type whereHelpernull_String struct{ field string }

func (w whereHelpernull_String) EQ(x null.String) qm.QueryMod {
	return qmhelper.WhereNullEQ(w.field, false, x)
}
func (w whereHelpernull_String) NEQ(x null.String) qm.QueryMod {
	return qmhelper.WhereNullEQ(w.field, true, x)
}
func (w whereHelpernull_String) LT(x null.String) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.LT, x)
}
func (w whereHelpernull_String) LTE(x null.String) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.LTE, x)
}
func (w whereHelpernull_String) GT(x null.String) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.GT, x)
}
func (w whereHelpernull_String) GTE(x null.String) qm.QueryMod {
	return qmhelper.Where(w.field, qmhelper.GTE, x)
}
func (w whereHelpernull_String) LIKE(x null.String) qm.QueryMod {
	return qm.Where(w.field+" LIKE ?", x)
}
func (w whereHelpernull_String) NLIKE(x null.String) qm.QueryMod {
	return qm.Where(w.field+" NOT LIKE ?", x)
}
func (w whereHelpernull_String) IN(slice []string) qm.QueryMod {
	values := make([]interface{}, 0, len(slice))
	for _, value := range slice {
		values = append(values, value)
	}
	return qm.WhereIn(fmt.Sprintf("%s IN ?", w.field), values...)
}
func (w whereHelpernull_String) NIN(slice []string) qm.QueryMod {
	values := make([]interface{}, 0, len(slice))
	for _, value := range slice {
		values = append(values, value)
	}
	return qm.WhereNotIn(fmt.Sprintf("%s NOT IN ?", w.field), values...)
}

func (w whereHelpernull_String) IsNull() qm.QueryMod    { return qmhelper.WhereIsNull(w.field) }
func (w whereHelpernull_String) IsNotNull() qm.QueryMod { return qmhelper.WhereIsNotNull(w.field) }

type whereHelper__byte struct{ field string }

func (w whereHelper__byte) EQ(x []byte) qm.QueryMod  { return qmhelper.Where(w.field, qmhelper.EQ, x) }
func (w whereHelper__byte) NEQ(x []byte) qm.QueryMod { return qmhelper.Where(w.field, qmhelper.NEQ, x) }
func (w whereHelper__byte) LT(x []byte) qm.QueryMod  { return qmhelper.Where(w.field, qmhelper.LT, x) }
func (w whereHelper__byte) LTE(x []byte) qm.QueryMod { return qmhelper.Where(w.field, qmhelper.LTE, x) }
func (w whereHelper__byte) GT(x []byte) qm.QueryMod  { return qmhelper.Where(w.field, qmhelper.GT, x) }
func (w whereHelper__byte) GTE(x []byte) qm.QueryMod { return qmhelper.Where(w.field, qmhelper.GTE, x) }

var SessionWhere = struct {
	ID           whereHelperstring
	CreatedAt    whereHelpertime_Time
	UpdatedAt    whereHelpertime_Time
	UserID       whereHelpernull_String
	ConnectionID whereHelpernull_String
	ExpiresAt    whereHelpertime_Time
	LastUsed     whereHelpertime_Time
	Deleted      whereHelperbool
	Meta         whereHelper__byte
}{
	ID:           whereHelperstring{field: "\"sessions\".\"id\""},
	CreatedAt:    whereHelpertime_Time{field: "\"sessions\".\"created_at\""},
	UpdatedAt:    whereHelpertime_Time{field: "\"sessions\".\"updated_at\""},
	UserID:       whereHelpernull_String{field: "\"sessions\".\"user_id\""},
	ConnectionID: whereHelpernull_String{field: "\"sessions\".\"connection_id\""},
	ExpiresAt:    whereHelpertime_Time{field: "\"sessions\".\"expires_at\""},
	LastUsed:     whereHelpertime_Time{field: "\"sessions\".\"last_used\""},
	Deleted:      whereHelperbool{field: "\"sessions\".\"deleted\""},
	Meta:         whereHelper__byte{field: "\"sessions\".\"meta\""},
}

// SessionRels is where relationship names are stored.
var SessionRels = struct {
}{}

// sessionR is where relationships are stored.
type sessionR struct {
}

// NewStruct creates a new relationship struct
func (*sessionR) NewStruct() *sessionR {
	return &sessionR{}
}

// sessionL is where Load methods for each relationship are stored.
type sessionL struct{}

var (
	sessionAllColumns            = []string{"id", "created_at", "updated_at", "user_id", "connection_id", "expires_at", "last_used", "deleted", "meta"}
	sessionColumnsWithoutDefault = []string{"id", "created_at", "updated_at", "expires_at", "last_used"}
	sessionColumnsWithDefault    = []string{"user_id", "connection_id", "deleted", "meta"}
	sessionPrimaryKeyColumns     = []string{"id"}
	sessionGeneratedColumns      = []string{}
)

type (
	// SessionSlice is an alias for a slice of pointers to Session.
	// This should almost always be used instead of []Session.
	SessionSlice []*Session
	// SessionHook is the signature for custom Session hook methods
	SessionHook func(context.Context, boil.ContextExecutor, *Session) error

	sessionQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	sessionType                 = reflect.TypeOf(&Session{})
	sessionMapping              = queries.MakeStructMapping(sessionType)
	sessionPrimaryKeyMapping, _ = queries.BindMapping(sessionType, sessionMapping, sessionPrimaryKeyColumns)
	sessionInsertCacheMut       sync.RWMutex
	sessionInsertCache          = make(map[string]insertCache)
	sessionUpdateCacheMut       sync.RWMutex
	sessionUpdateCache          = make(map[string]updateCache)
	sessionUpsertCacheMut       sync.RWMutex
	sessionUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force qmhelper dependency for where clause generation (which doesn't
	// always happen)
	_ = qmhelper.Where
)

var sessionAfterSelectMu sync.Mutex
var sessionAfterSelectHooks []SessionHook

var sessionBeforeInsertMu sync.Mutex
var sessionBeforeInsertHooks []SessionHook
var sessionAfterInsertMu sync.Mutex
var sessionAfterInsertHooks []SessionHook

var sessionBeforeUpdateMu sync.Mutex
var sessionBeforeUpdateHooks []SessionHook
var sessionAfterUpdateMu sync.Mutex
var sessionAfterUpdateHooks []SessionHook

var sessionBeforeDeleteMu sync.Mutex
var sessionBeforeDeleteHooks []SessionHook
var sessionAfterDeleteMu sync.Mutex
var sessionAfterDeleteHooks []SessionHook

var sessionBeforeUpsertMu sync.Mutex
var sessionBeforeUpsertHooks []SessionHook
var sessionAfterUpsertMu sync.Mutex
var sessionAfterUpsertHooks []SessionHook

// doAfterSelectHooks executes all "after Select" hooks.
func (o *Session) doAfterSelectHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range sessionAfterSelectHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeInsertHooks executes all "before insert" hooks.
func (o *Session) doBeforeInsertHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range sessionBeforeInsertHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterInsertHooks executes all "after Insert" hooks.
func (o *Session) doAfterInsertHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range sessionAfterInsertHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeUpdateHooks executes all "before Update" hooks.
func (o *Session) doBeforeUpdateHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range sessionBeforeUpdateHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterUpdateHooks executes all "after Update" hooks.
func (o *Session) doAfterUpdateHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range sessionAfterUpdateHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeDeleteHooks executes all "before Delete" hooks.
func (o *Session) doBeforeDeleteHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range sessionBeforeDeleteHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterDeleteHooks executes all "after Delete" hooks.
func (o *Session) doAfterDeleteHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range sessionAfterDeleteHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeUpsertHooks executes all "before Upsert" hooks.
func (o *Session) doBeforeUpsertHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range sessionBeforeUpsertHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterUpsertHooks executes all "after Upsert" hooks.
func (o *Session) doAfterUpsertHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range sessionAfterUpsertHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// AddSessionHook registers your hook function for all future operations.
func AddSessionHook(hookPoint boil.HookPoint, sessionHook SessionHook) {
	switch hookPoint {
	case boil.AfterSelectHook:
		sessionAfterSelectMu.Lock()
		sessionAfterSelectHooks = append(sessionAfterSelectHooks, sessionHook)
		sessionAfterSelectMu.Unlock()
	case boil.BeforeInsertHook:
		sessionBeforeInsertMu.Lock()
		sessionBeforeInsertHooks = append(sessionBeforeInsertHooks, sessionHook)
		sessionBeforeInsertMu.Unlock()
	case boil.AfterInsertHook:
		sessionAfterInsertMu.Lock()
		sessionAfterInsertHooks = append(sessionAfterInsertHooks, sessionHook)
		sessionAfterInsertMu.Unlock()
	case boil.BeforeUpdateHook:
		sessionBeforeUpdateMu.Lock()
		sessionBeforeUpdateHooks = append(sessionBeforeUpdateHooks, sessionHook)
		sessionBeforeUpdateMu.Unlock()
	case boil.AfterUpdateHook:
		sessionAfterUpdateMu.Lock()
		sessionAfterUpdateHooks = append(sessionAfterUpdateHooks, sessionHook)
		sessionAfterUpdateMu.Unlock()
	case boil.BeforeDeleteHook:
		sessionBeforeDeleteMu.Lock()
		sessionBeforeDeleteHooks = append(sessionBeforeDeleteHooks, sessionHook)
		sessionBeforeDeleteMu.Unlock()
	case boil.AfterDeleteHook:
		sessionAfterDeleteMu.Lock()
		sessionAfterDeleteHooks = append(sessionAfterDeleteHooks, sessionHook)
		sessionAfterDeleteMu.Unlock()
	case boil.BeforeUpsertHook:
		sessionBeforeUpsertMu.Lock()
		sessionBeforeUpsertHooks = append(sessionBeforeUpsertHooks, sessionHook)
		sessionBeforeUpsertMu.Unlock()
	case boil.AfterUpsertHook:
		sessionAfterUpsertMu.Lock()
		sessionAfterUpsertHooks = append(sessionAfterUpsertHooks, sessionHook)
		sessionAfterUpsertMu.Unlock()
	}
}

// One returns a single session record from the query.
func (q sessionQuery) One(ctx context.Context, exec boil.ContextExecutor) (*Session, error) {
	o := &Session{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(ctx, exec, o)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: failed to execute a one query for sessions")
	}

	if err := o.doAfterSelectHooks(ctx, exec); err != nil {
		return o, err
	}

	return o, nil
}

// All returns all Session records from the query.
func (q sessionQuery) All(ctx context.Context, exec boil.ContextExecutor) (SessionSlice, error) {
	var o []*Session

	err := q.Bind(ctx, exec, &o)
	if err != nil {
		return nil, errors.Wrap(err, "models: failed to assign all query results to Session slice")
	}

	if len(sessionAfterSelectHooks) != 0 {
		for _, obj := range o {
			if err := obj.doAfterSelectHooks(ctx, exec); err != nil {
				return o, err
			}
		}
	}

	return o, nil
}

// Count returns the count of all Session records in the query.
func (q sessionQuery) Count(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRowContext(ctx, exec).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to count sessions rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table.
func (q sessionQuery) Exists(ctx context.Context, exec boil.ContextExecutor) (bool, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRowContext(ctx, exec).Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "models: failed to check if sessions exists")
	}

	return count > 0, nil
}

// Sessions retrieves all the records using an executor.
func Sessions(mods ...qm.QueryMod) sessionQuery {
	mods = append(mods, qm.From("\"sessions\""))
	q := NewQuery(mods...)
	if len(queries.GetSelect(q)) == 0 {
		queries.SetSelect(q, []string{"\"sessions\".*"})
	}

	return sessionQuery{q}
}

// FindSession retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindSession(ctx context.Context, exec boil.ContextExecutor, iD string, selectCols ...string) (*Session, error) {
	sessionObj := &Session{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"sessions\" where \"id\"=?", sel,
	)

	q := queries.Raw(query, iD)

	err := q.Bind(ctx, exec, sessionObj)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: unable to select from sessions")
	}

	if err = sessionObj.doAfterSelectHooks(ctx, exec); err != nil {
		return sessionObj, err
	}

	return sessionObj, nil
}

// Insert a single record using an executor.
// See boil.Columns.InsertColumnSet documentation to understand column list inference for inserts.
func (o *Session) Insert(ctx context.Context, exec boil.ContextExecutor, columns boil.Columns) error {
	if o == nil {
		return errors.New("models: no sessions provided for insertion")
	}

	var err error
	if !boil.TimestampsAreSkipped(ctx) {
		currTime := time.Now().In(boil.GetLocation())

		if o.CreatedAt.IsZero() {
			o.CreatedAt = currTime
		}
		if o.UpdatedAt.IsZero() {
			o.UpdatedAt = currTime
		}
	}

	if err := o.doBeforeInsertHooks(ctx, exec); err != nil {
		return err
	}

	nzDefaults := queries.NonZeroDefaultSet(sessionColumnsWithDefault, o)

	key := makeCacheKey(columns, nzDefaults)
	sessionInsertCacheMut.RLock()
	cache, cached := sessionInsertCache[key]
	sessionInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := columns.InsertColumnSet(
			sessionAllColumns,
			sessionColumnsWithDefault,
			sessionColumnsWithoutDefault,
			nzDefaults,
		)

		cache.valueMapping, err = queries.BindMapping(sessionType, sessionMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(sessionType, sessionMapping, returnColumns)
		if err != nil {
			return err
		}
		if len(wl) != 0 {
			cache.query = fmt.Sprintf("INSERT INTO \"sessions\" (\"%s\") %%sVALUES (%s)%%s", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.UseIndexPlaceholders, len(wl), 1, 1))
		} else {
			cache.query = "INSERT INTO \"sessions\" %sDEFAULT VALUES%s"
		}

		var queryOutput, queryReturning string

		if len(cache.retMapping) != 0 {
			queryReturning = fmt.Sprintf(" RETURNING \"%s\"", strings.Join(returnColumns, "\",\""))
		}

		cache.query = fmt.Sprintf(cache.query, queryOutput, queryReturning)
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, cache.query)
		fmt.Fprintln(writer, vals)
	}

	if len(cache.retMapping) != 0 {
		err = exec.QueryRowContext(ctx, cache.query, vals...).Scan(queries.PtrsFromMapping(value, cache.retMapping)...)
	} else {
		_, err = exec.ExecContext(ctx, cache.query, vals...)
	}

	if err != nil {
		return errors.Wrap(err, "models: unable to insert into sessions")
	}

	if !cached {
		sessionInsertCacheMut.Lock()
		sessionInsertCache[key] = cache
		sessionInsertCacheMut.Unlock()
	}

	return o.doAfterInsertHooks(ctx, exec)
}

// Update uses an executor to update the Session.
// See boil.Columns.UpdateColumnSet documentation to understand column list inference for updates.
// Update does not automatically update the record in case of default values. Use .Reload() to refresh the records.
func (o *Session) Update(ctx context.Context, exec boil.ContextExecutor, columns boil.Columns) (int64, error) {
	if !boil.TimestampsAreSkipped(ctx) {
		currTime := time.Now().In(boil.GetLocation())

		o.UpdatedAt = currTime
	}

	var err error
	if err = o.doBeforeUpdateHooks(ctx, exec); err != nil {
		return 0, err
	}
	key := makeCacheKey(columns, nil)
	sessionUpdateCacheMut.RLock()
	cache, cached := sessionUpdateCache[key]
	sessionUpdateCacheMut.RUnlock()

	if !cached {
		wl := columns.UpdateColumnSet(
			sessionAllColumns,
			sessionPrimaryKeyColumns,
		)

		if !columns.IsWhitelist() {
			wl = strmangle.SetComplement(wl, []string{"created_at"})
		}
		if len(wl) == 0 {
			return 0, errors.New("models: unable to update sessions, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"sessions\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 0, wl),
			strmangle.WhereClause("\"", "\"", 0, sessionPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(sessionType, sessionMapping, append(wl, sessionPrimaryKeyColumns...))
		if err != nil {
			return 0, err
		}
	}

	values := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), cache.valueMapping)

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, cache.query)
		fmt.Fprintln(writer, values)
	}
	var result sql.Result
	result, err = exec.ExecContext(ctx, cache.query, values...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update sessions row")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by update for sessions")
	}

	if !cached {
		sessionUpdateCacheMut.Lock()
		sessionUpdateCache[key] = cache
		sessionUpdateCacheMut.Unlock()
	}

	return rowsAff, o.doAfterUpdateHooks(ctx, exec)
}

// UpdateAll updates all rows with the specified column values.
func (q sessionQuery) UpdateAll(ctx context.Context, exec boil.ContextExecutor, cols M) (int64, error) {
	queries.SetUpdate(q.Query, cols)

	result, err := q.Query.ExecContext(ctx, exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all for sessions")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected for sessions")
	}

	return rowsAff, nil
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o SessionSlice) UpdateAll(ctx context.Context, exec boil.ContextExecutor, cols M) (int64, error) {
	ln := int64(len(o))
	if ln == 0 {
		return 0, nil
	}

	if len(cols) == 0 {
		return 0, errors.New("models: update all requires at least one column argument")
	}

	colNames := make([]string, len(cols))
	args := make([]interface{}, len(cols))

	i := 0
	for name, value := range cols {
		colNames[i] = name
		args[i] = value
		i++
	}

	// Append all of the primary key values for each column
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), sessionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf("UPDATE \"sessions\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 0, colNames),
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 0, sessionPrimaryKeyColumns, len(o)))

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, args...)
	}
	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all in session slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected all in update all session")
	}
	return rowsAff, nil
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
// See boil.Columns documentation for how to properly use updateColumns and insertColumns.
func (o *Session) Upsert(ctx context.Context, exec boil.ContextExecutor, updateOnConflict bool, conflictColumns []string, updateColumns, insertColumns boil.Columns) error {
	if o == nil {
		return errors.New("models: no sessions provided for upsert")
	}
	if !boil.TimestampsAreSkipped(ctx) {
		currTime := time.Now().In(boil.GetLocation())

		if o.CreatedAt.IsZero() {
			o.CreatedAt = currTime
		}
		o.UpdatedAt = currTime
	}

	if err := o.doBeforeUpsertHooks(ctx, exec); err != nil {
		return err
	}

	nzDefaults := queries.NonZeroDefaultSet(sessionColumnsWithDefault, o)

	// Build cache key in-line uglily - mysql vs psql problems
	buf := strmangle.GetBuffer()
	if updateOnConflict {
		buf.WriteByte('t')
	} else {
		buf.WriteByte('f')
	}
	buf.WriteByte('.')
	for _, c := range conflictColumns {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	buf.WriteString(strconv.Itoa(updateColumns.Kind))
	for _, c := range updateColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	buf.WriteString(strconv.Itoa(insertColumns.Kind))
	for _, c := range insertColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range nzDefaults {
		buf.WriteString(c)
	}
	key := buf.String()
	strmangle.PutBuffer(buf)

	sessionUpsertCacheMut.RLock()
	cache, cached := sessionUpsertCache[key]
	sessionUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		insert, _ := insertColumns.InsertColumnSet(
			sessionAllColumns,
			sessionColumnsWithDefault,
			sessionColumnsWithoutDefault,
			nzDefaults,
		)
		update := updateColumns.UpdateColumnSet(
			sessionAllColumns,
			sessionPrimaryKeyColumns,
		)

		if updateOnConflict && len(update) == 0 {
			return errors.New("models: unable to upsert sessions, could not build update column list")
		}

		ret := strmangle.SetComplement(sessionAllColumns, strmangle.SetIntersect(insert, update))

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(sessionPrimaryKeyColumns))
			copy(conflict, sessionPrimaryKeyColumns)
		}
		cache.query = buildUpsertQuerySQLite(dialect, "\"sessions\"", updateOnConflict, ret, update, conflict, insert)

		cache.valueMapping, err = queries.BindMapping(sessionType, sessionMapping, insert)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(sessionType, sessionMapping, ret)
			if err != nil {
				return err
			}
		}
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)
	var returns []interface{}
	if len(cache.retMapping) != 0 {
		returns = queries.PtrsFromMapping(value, cache.retMapping)
	}

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, cache.query)
		fmt.Fprintln(writer, vals)
	}
	if len(cache.retMapping) != 0 {
		err = exec.QueryRowContext(ctx, cache.query, vals...).Scan(returns...)
		if errors.Is(err, sql.ErrNoRows) {
			err = nil // Postgres doesn't return anything when there's no update
		}
	} else {
		_, err = exec.ExecContext(ctx, cache.query, vals...)
	}
	if err != nil {
		return errors.Wrap(err, "models: unable to upsert sessions")
	}

	if !cached {
		sessionUpsertCacheMut.Lock()
		sessionUpsertCache[key] = cache
		sessionUpsertCacheMut.Unlock()
	}

	return o.doAfterUpsertHooks(ctx, exec)
}

// Delete deletes a single Session record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Session) Delete(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if o == nil {
		return 0, errors.New("models: no Session provided for delete")
	}

	if err := o.doBeforeDeleteHooks(ctx, exec); err != nil {
		return 0, err
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), sessionPrimaryKeyMapping)
	sql := "DELETE FROM \"sessions\" WHERE \"id\"=?"

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, args...)
	}
	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete from sessions")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by delete for sessions")
	}

	if err := o.doAfterDeleteHooks(ctx, exec); err != nil {
		return 0, err
	}

	return rowsAff, nil
}

// DeleteAll deletes all matching rows.
func (q sessionQuery) DeleteAll(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if q.Query == nil {
		return 0, errors.New("models: no sessionQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	result, err := q.Query.ExecContext(ctx, exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from sessions")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for sessions")
	}

	return rowsAff, nil
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o SessionSlice) DeleteAll(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if len(o) == 0 {
		return 0, nil
	}

	if len(sessionBeforeDeleteHooks) != 0 {
		for _, obj := range o {
			if err := obj.doBeforeDeleteHooks(ctx, exec); err != nil {
				return 0, err
			}
		}
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), sessionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "DELETE FROM \"sessions\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 0, sessionPrimaryKeyColumns, len(o))

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, args)
	}
	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from session slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for sessions")
	}

	if len(sessionAfterDeleteHooks) != 0 {
		for _, obj := range o {
			if err := obj.doAfterDeleteHooks(ctx, exec); err != nil {
				return 0, err
			}
		}
	}

	return rowsAff, nil
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *Session) Reload(ctx context.Context, exec boil.ContextExecutor) error {
	ret, err := FindSession(ctx, exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *SessionSlice) ReloadAll(ctx context.Context, exec boil.ContextExecutor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	slice := SessionSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), sessionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "SELECT \"sessions\".* FROM \"sessions\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 0, sessionPrimaryKeyColumns, len(*o))

	q := queries.Raw(sql, args...)

	err := q.Bind(ctx, exec, &slice)
	if err != nil {
		return errors.Wrap(err, "models: unable to reload all in SessionSlice")
	}

	*o = slice

	return nil
}

// SessionExists checks if the Session row exists.
func SessionExists(ctx context.Context, exec boil.ContextExecutor, iD string) (bool, error) {
	var exists bool
	sql := "select exists(select 1 from \"sessions\" where \"id\"=? limit 1)"

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, iD)
	}
	row := exec.QueryRowContext(ctx, sql, iD)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "models: unable to check if sessions exists")
	}

	return exists, nil
}

// Exists checks if the Session row exists.
func (o *Session) Exists(ctx context.Context, exec boil.ContextExecutor) (bool, error) {
	return SessionExists(ctx, exec, o.ID)
}
