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
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"github.com/volatiletech/sqlboiler/v4/queries/qmhelper"
	"github.com/volatiletech/strmangle"
)

// Passkey is an object representing the database table.
type Passkey struct {
	ID        string    `boil:"id" json:"id" toml:"id" yaml:"id"`
	CreatedAt time.Time `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	UpdatedAt time.Time `boil:"updated_at" json:"updated_at" toml:"updated_at" yaml:"updated_at"`
	Label     string    `boil:"label" json:"label" toml:"label" yaml:"label"`
	Data      []byte    `boil:"data" json:"data" toml:"data" yaml:"data"`
	UserID    string    `boil:"user_id" json:"user_id" toml:"user_id" yaml:"user_id"`

	R *passkeyR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L passkeyL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

var PasskeyColumns = struct {
	ID        string
	CreatedAt string
	UpdatedAt string
	Label     string
	Data      string
	UserID    string
}{
	ID:        "id",
	CreatedAt: "created_at",
	UpdatedAt: "updated_at",
	Label:     "label",
	Data:      "data",
	UserID:    "user_id",
}

var PasskeyTableColumns = struct {
	ID        string
	CreatedAt string
	UpdatedAt string
	Label     string
	Data      string
	UserID    string
}{
	ID:        "passkeys.id",
	CreatedAt: "passkeys.created_at",
	UpdatedAt: "passkeys.updated_at",
	Label:     "passkeys.label",
	Data:      "passkeys.data",
	UserID:    "passkeys.user_id",
}

// Generated where

type whereHelper__byte struct{ field string }

func (w whereHelper__byte) EQ(x []byte) qm.QueryMod  { return qmhelper.Where(w.field, qmhelper.EQ, x) }
func (w whereHelper__byte) NEQ(x []byte) qm.QueryMod { return qmhelper.Where(w.field, qmhelper.NEQ, x) }
func (w whereHelper__byte) LT(x []byte) qm.QueryMod  { return qmhelper.Where(w.field, qmhelper.LT, x) }
func (w whereHelper__byte) LTE(x []byte) qm.QueryMod { return qmhelper.Where(w.field, qmhelper.LTE, x) }
func (w whereHelper__byte) GT(x []byte) qm.QueryMod  { return qmhelper.Where(w.field, qmhelper.GT, x) }
func (w whereHelper__byte) GTE(x []byte) qm.QueryMod { return qmhelper.Where(w.field, qmhelper.GTE, x) }

var PasskeyWhere = struct {
	ID        whereHelperstring
	CreatedAt whereHelpertime_Time
	UpdatedAt whereHelpertime_Time
	Label     whereHelperstring
	Data      whereHelper__byte
	UserID    whereHelperstring
}{
	ID:        whereHelperstring{field: "\"passkeys\".\"id\""},
	CreatedAt: whereHelpertime_Time{field: "\"passkeys\".\"created_at\""},
	UpdatedAt: whereHelpertime_Time{field: "\"passkeys\".\"updated_at\""},
	Label:     whereHelperstring{field: "\"passkeys\".\"label\""},
	Data:      whereHelper__byte{field: "\"passkeys\".\"data\""},
	UserID:    whereHelperstring{field: "\"passkeys\".\"user_id\""},
}

// PasskeyRels is where relationship names are stored.
var PasskeyRels = struct {
}{}

// passkeyR is where relationships are stored.
type passkeyR struct {
}

// NewStruct creates a new relationship struct
func (*passkeyR) NewStruct() *passkeyR {
	return &passkeyR{}
}

// passkeyL is where Load methods for each relationship are stored.
type passkeyL struct{}

var (
	passkeyAllColumns            = []string{"id", "created_at", "updated_at", "label", "data", "user_id"}
	passkeyColumnsWithoutDefault = []string{"id", "created_at", "updated_at", "data", "user_id"}
	passkeyColumnsWithDefault    = []string{"label"}
	passkeyPrimaryKeyColumns     = []string{"id"}
	passkeyGeneratedColumns      = []string{}
)

type (
	// PasskeySlice is an alias for a slice of pointers to Passkey.
	// This should almost always be used instead of []Passkey.
	PasskeySlice []*Passkey
	// PasskeyHook is the signature for custom Passkey hook methods
	PasskeyHook func(context.Context, boil.ContextExecutor, *Passkey) error

	passkeyQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	passkeyType                 = reflect.TypeOf(&Passkey{})
	passkeyMapping              = queries.MakeStructMapping(passkeyType)
	passkeyPrimaryKeyMapping, _ = queries.BindMapping(passkeyType, passkeyMapping, passkeyPrimaryKeyColumns)
	passkeyInsertCacheMut       sync.RWMutex
	passkeyInsertCache          = make(map[string]insertCache)
	passkeyUpdateCacheMut       sync.RWMutex
	passkeyUpdateCache          = make(map[string]updateCache)
	passkeyUpsertCacheMut       sync.RWMutex
	passkeyUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force qmhelper dependency for where clause generation (which doesn't
	// always happen)
	_ = qmhelper.Where
)

var passkeyAfterSelectMu sync.Mutex
var passkeyAfterSelectHooks []PasskeyHook

var passkeyBeforeInsertMu sync.Mutex
var passkeyBeforeInsertHooks []PasskeyHook
var passkeyAfterInsertMu sync.Mutex
var passkeyAfterInsertHooks []PasskeyHook

var passkeyBeforeUpdateMu sync.Mutex
var passkeyBeforeUpdateHooks []PasskeyHook
var passkeyAfterUpdateMu sync.Mutex
var passkeyAfterUpdateHooks []PasskeyHook

var passkeyBeforeDeleteMu sync.Mutex
var passkeyBeforeDeleteHooks []PasskeyHook
var passkeyAfterDeleteMu sync.Mutex
var passkeyAfterDeleteHooks []PasskeyHook

var passkeyBeforeUpsertMu sync.Mutex
var passkeyBeforeUpsertHooks []PasskeyHook
var passkeyAfterUpsertMu sync.Mutex
var passkeyAfterUpsertHooks []PasskeyHook

// doAfterSelectHooks executes all "after Select" hooks.
func (o *Passkey) doAfterSelectHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range passkeyAfterSelectHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeInsertHooks executes all "before insert" hooks.
func (o *Passkey) doBeforeInsertHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range passkeyBeforeInsertHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterInsertHooks executes all "after Insert" hooks.
func (o *Passkey) doAfterInsertHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range passkeyAfterInsertHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeUpdateHooks executes all "before Update" hooks.
func (o *Passkey) doBeforeUpdateHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range passkeyBeforeUpdateHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterUpdateHooks executes all "after Update" hooks.
func (o *Passkey) doAfterUpdateHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range passkeyAfterUpdateHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeDeleteHooks executes all "before Delete" hooks.
func (o *Passkey) doBeforeDeleteHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range passkeyBeforeDeleteHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterDeleteHooks executes all "after Delete" hooks.
func (o *Passkey) doAfterDeleteHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range passkeyAfterDeleteHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeUpsertHooks executes all "before Upsert" hooks.
func (o *Passkey) doBeforeUpsertHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range passkeyBeforeUpsertHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterUpsertHooks executes all "after Upsert" hooks.
func (o *Passkey) doAfterUpsertHooks(ctx context.Context, exec boil.ContextExecutor) (err error) {
	if boil.HooksAreSkipped(ctx) {
		return nil
	}

	for _, hook := range passkeyAfterUpsertHooks {
		if err := hook(ctx, exec, o); err != nil {
			return err
		}
	}

	return nil
}

// AddPasskeyHook registers your hook function for all future operations.
func AddPasskeyHook(hookPoint boil.HookPoint, passkeyHook PasskeyHook) {
	switch hookPoint {
	case boil.AfterSelectHook:
		passkeyAfterSelectMu.Lock()
		passkeyAfterSelectHooks = append(passkeyAfterSelectHooks, passkeyHook)
		passkeyAfterSelectMu.Unlock()
	case boil.BeforeInsertHook:
		passkeyBeforeInsertMu.Lock()
		passkeyBeforeInsertHooks = append(passkeyBeforeInsertHooks, passkeyHook)
		passkeyBeforeInsertMu.Unlock()
	case boil.AfterInsertHook:
		passkeyAfterInsertMu.Lock()
		passkeyAfterInsertHooks = append(passkeyAfterInsertHooks, passkeyHook)
		passkeyAfterInsertMu.Unlock()
	case boil.BeforeUpdateHook:
		passkeyBeforeUpdateMu.Lock()
		passkeyBeforeUpdateHooks = append(passkeyBeforeUpdateHooks, passkeyHook)
		passkeyBeforeUpdateMu.Unlock()
	case boil.AfterUpdateHook:
		passkeyAfterUpdateMu.Lock()
		passkeyAfterUpdateHooks = append(passkeyAfterUpdateHooks, passkeyHook)
		passkeyAfterUpdateMu.Unlock()
	case boil.BeforeDeleteHook:
		passkeyBeforeDeleteMu.Lock()
		passkeyBeforeDeleteHooks = append(passkeyBeforeDeleteHooks, passkeyHook)
		passkeyBeforeDeleteMu.Unlock()
	case boil.AfterDeleteHook:
		passkeyAfterDeleteMu.Lock()
		passkeyAfterDeleteHooks = append(passkeyAfterDeleteHooks, passkeyHook)
		passkeyAfterDeleteMu.Unlock()
	case boil.BeforeUpsertHook:
		passkeyBeforeUpsertMu.Lock()
		passkeyBeforeUpsertHooks = append(passkeyBeforeUpsertHooks, passkeyHook)
		passkeyBeforeUpsertMu.Unlock()
	case boil.AfterUpsertHook:
		passkeyAfterUpsertMu.Lock()
		passkeyAfterUpsertHooks = append(passkeyAfterUpsertHooks, passkeyHook)
		passkeyAfterUpsertMu.Unlock()
	}
}

// One returns a single passkey record from the query.
func (q passkeyQuery) One(ctx context.Context, exec boil.ContextExecutor) (*Passkey, error) {
	o := &Passkey{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(ctx, exec, o)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: failed to execute a one query for passkeys")
	}

	if err := o.doAfterSelectHooks(ctx, exec); err != nil {
		return o, err
	}

	return o, nil
}

// All returns all Passkey records from the query.
func (q passkeyQuery) All(ctx context.Context, exec boil.ContextExecutor) (PasskeySlice, error) {
	var o []*Passkey

	err := q.Bind(ctx, exec, &o)
	if err != nil {
		return nil, errors.Wrap(err, "models: failed to assign all query results to Passkey slice")
	}

	if len(passkeyAfterSelectHooks) != 0 {
		for _, obj := range o {
			if err := obj.doAfterSelectHooks(ctx, exec); err != nil {
				return o, err
			}
		}
	}

	return o, nil
}

// Count returns the count of all Passkey records in the query.
func (q passkeyQuery) Count(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRowContext(ctx, exec).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to count passkeys rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table.
func (q passkeyQuery) Exists(ctx context.Context, exec boil.ContextExecutor) (bool, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRowContext(ctx, exec).Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "models: failed to check if passkeys exists")
	}

	return count > 0, nil
}

// Passkeys retrieves all the records using an executor.
func Passkeys(mods ...qm.QueryMod) passkeyQuery {
	mods = append(mods, qm.From("\"passkeys\""))
	q := NewQuery(mods...)
	if len(queries.GetSelect(q)) == 0 {
		queries.SetSelect(q, []string{"\"passkeys\".*"})
	}

	return passkeyQuery{q}
}

// FindPasskey retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindPasskey(ctx context.Context, exec boil.ContextExecutor, iD string, selectCols ...string) (*Passkey, error) {
	passkeyObj := &Passkey{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"passkeys\" where \"id\"=?", sel,
	)

	q := queries.Raw(query, iD)

	err := q.Bind(ctx, exec, passkeyObj)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: unable to select from passkeys")
	}

	if err = passkeyObj.doAfterSelectHooks(ctx, exec); err != nil {
		return passkeyObj, err
	}

	return passkeyObj, nil
}

// Insert a single record using an executor.
// See boil.Columns.InsertColumnSet documentation to understand column list inference for inserts.
func (o *Passkey) Insert(ctx context.Context, exec boil.ContextExecutor, columns boil.Columns) error {
	if o == nil {
		return errors.New("models: no passkeys provided for insertion")
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

	nzDefaults := queries.NonZeroDefaultSet(passkeyColumnsWithDefault, o)

	key := makeCacheKey(columns, nzDefaults)
	passkeyInsertCacheMut.RLock()
	cache, cached := passkeyInsertCache[key]
	passkeyInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := columns.InsertColumnSet(
			passkeyAllColumns,
			passkeyColumnsWithDefault,
			passkeyColumnsWithoutDefault,
			nzDefaults,
		)

		cache.valueMapping, err = queries.BindMapping(passkeyType, passkeyMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(passkeyType, passkeyMapping, returnColumns)
		if err != nil {
			return err
		}
		if len(wl) != 0 {
			cache.query = fmt.Sprintf("INSERT INTO \"passkeys\" (\"%s\") %%sVALUES (%s)%%s", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.UseIndexPlaceholders, len(wl), 1, 1))
		} else {
			cache.query = "INSERT INTO \"passkeys\" %sDEFAULT VALUES%s"
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
		return errors.Wrap(err, "models: unable to insert into passkeys")
	}

	if !cached {
		passkeyInsertCacheMut.Lock()
		passkeyInsertCache[key] = cache
		passkeyInsertCacheMut.Unlock()
	}

	return o.doAfterInsertHooks(ctx, exec)
}

// Update uses an executor to update the Passkey.
// See boil.Columns.UpdateColumnSet documentation to understand column list inference for updates.
// Update does not automatically update the record in case of default values. Use .Reload() to refresh the records.
func (o *Passkey) Update(ctx context.Context, exec boil.ContextExecutor, columns boil.Columns) (int64, error) {
	if !boil.TimestampsAreSkipped(ctx) {
		currTime := time.Now().In(boil.GetLocation())

		o.UpdatedAt = currTime
	}

	var err error
	if err = o.doBeforeUpdateHooks(ctx, exec); err != nil {
		return 0, err
	}
	key := makeCacheKey(columns, nil)
	passkeyUpdateCacheMut.RLock()
	cache, cached := passkeyUpdateCache[key]
	passkeyUpdateCacheMut.RUnlock()

	if !cached {
		wl := columns.UpdateColumnSet(
			passkeyAllColumns,
			passkeyPrimaryKeyColumns,
		)

		if !columns.IsWhitelist() {
			wl = strmangle.SetComplement(wl, []string{"created_at"})
		}
		if len(wl) == 0 {
			return 0, errors.New("models: unable to update passkeys, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"passkeys\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 0, wl),
			strmangle.WhereClause("\"", "\"", 0, passkeyPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(passkeyType, passkeyMapping, append(wl, passkeyPrimaryKeyColumns...))
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
		return 0, errors.Wrap(err, "models: unable to update passkeys row")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by update for passkeys")
	}

	if !cached {
		passkeyUpdateCacheMut.Lock()
		passkeyUpdateCache[key] = cache
		passkeyUpdateCacheMut.Unlock()
	}

	return rowsAff, o.doAfterUpdateHooks(ctx, exec)
}

// UpdateAll updates all rows with the specified column values.
func (q passkeyQuery) UpdateAll(ctx context.Context, exec boil.ContextExecutor, cols M) (int64, error) {
	queries.SetUpdate(q.Query, cols)

	result, err := q.Query.ExecContext(ctx, exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all for passkeys")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected for passkeys")
	}

	return rowsAff, nil
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o PasskeySlice) UpdateAll(ctx context.Context, exec boil.ContextExecutor, cols M) (int64, error) {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), passkeyPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf("UPDATE \"passkeys\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 0, colNames),
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 0, passkeyPrimaryKeyColumns, len(o)))

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, args...)
	}
	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all in passkey slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected all in update all passkey")
	}
	return rowsAff, nil
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
// See boil.Columns documentation for how to properly use updateColumns and insertColumns.
func (o *Passkey) Upsert(ctx context.Context, exec boil.ContextExecutor, updateOnConflict bool, conflictColumns []string, updateColumns, insertColumns boil.Columns) error {
	if o == nil {
		return errors.New("models: no passkeys provided for upsert")
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

	nzDefaults := queries.NonZeroDefaultSet(passkeyColumnsWithDefault, o)

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

	passkeyUpsertCacheMut.RLock()
	cache, cached := passkeyUpsertCache[key]
	passkeyUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		insert, _ := insertColumns.InsertColumnSet(
			passkeyAllColumns,
			passkeyColumnsWithDefault,
			passkeyColumnsWithoutDefault,
			nzDefaults,
		)
		update := updateColumns.UpdateColumnSet(
			passkeyAllColumns,
			passkeyPrimaryKeyColumns,
		)

		if updateOnConflict && len(update) == 0 {
			return errors.New("models: unable to upsert passkeys, could not build update column list")
		}

		ret := strmangle.SetComplement(passkeyAllColumns, strmangle.SetIntersect(insert, update))

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(passkeyPrimaryKeyColumns))
			copy(conflict, passkeyPrimaryKeyColumns)
		}
		cache.query = buildUpsertQuerySQLite(dialect, "\"passkeys\"", updateOnConflict, ret, update, conflict, insert)

		cache.valueMapping, err = queries.BindMapping(passkeyType, passkeyMapping, insert)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(passkeyType, passkeyMapping, ret)
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
		return errors.Wrap(err, "models: unable to upsert passkeys")
	}

	if !cached {
		passkeyUpsertCacheMut.Lock()
		passkeyUpsertCache[key] = cache
		passkeyUpsertCacheMut.Unlock()
	}

	return o.doAfterUpsertHooks(ctx, exec)
}

// Delete deletes a single Passkey record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Passkey) Delete(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if o == nil {
		return 0, errors.New("models: no Passkey provided for delete")
	}

	if err := o.doBeforeDeleteHooks(ctx, exec); err != nil {
		return 0, err
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), passkeyPrimaryKeyMapping)
	sql := "DELETE FROM \"passkeys\" WHERE \"id\"=?"

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, args...)
	}
	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete from passkeys")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by delete for passkeys")
	}

	if err := o.doAfterDeleteHooks(ctx, exec); err != nil {
		return 0, err
	}

	return rowsAff, nil
}

// DeleteAll deletes all matching rows.
func (q passkeyQuery) DeleteAll(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if q.Query == nil {
		return 0, errors.New("models: no passkeyQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	result, err := q.Query.ExecContext(ctx, exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from passkeys")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for passkeys")
	}

	return rowsAff, nil
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o PasskeySlice) DeleteAll(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if len(o) == 0 {
		return 0, nil
	}

	if len(passkeyBeforeDeleteHooks) != 0 {
		for _, obj := range o {
			if err := obj.doBeforeDeleteHooks(ctx, exec); err != nil {
				return 0, err
			}
		}
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), passkeyPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "DELETE FROM \"passkeys\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 0, passkeyPrimaryKeyColumns, len(o))

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, args)
	}
	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from passkey slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for passkeys")
	}

	if len(passkeyAfterDeleteHooks) != 0 {
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
func (o *Passkey) Reload(ctx context.Context, exec boil.ContextExecutor) error {
	ret, err := FindPasskey(ctx, exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *PasskeySlice) ReloadAll(ctx context.Context, exec boil.ContextExecutor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	slice := PasskeySlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), passkeyPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "SELECT \"passkeys\".* FROM \"passkeys\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 0, passkeyPrimaryKeyColumns, len(*o))

	q := queries.Raw(sql, args...)

	err := q.Bind(ctx, exec, &slice)
	if err != nil {
		return errors.Wrap(err, "models: unable to reload all in PasskeySlice")
	}

	*o = slice

	return nil
}

// PasskeyExists checks if the Passkey row exists.
func PasskeyExists(ctx context.Context, exec boil.ContextExecutor, iD string) (bool, error) {
	var exists bool
	sql := "select exists(select 1 from \"passkeys\" where \"id\"=? limit 1)"

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, iD)
	}
	row := exec.QueryRowContext(ctx, sql, iD)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "models: unable to check if passkeys exists")
	}

	return exists, nil
}

// Exists checks if the Passkey row exists.
func (o *Passkey) Exists(ctx context.Context, exec boil.ContextExecutor) (bool, error) {
	return PasskeyExists(ctx, exec, o.ID)
}
