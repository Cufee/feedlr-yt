// Code generated by SQLBoiler 4.16.2 (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/volatiletech/randomize"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/strmangle"
)

var (
	// Relationships sometimes use the reflection helper queries.Equal/queries.Assign
	// so force a package dependency in case they don't.
	_ = queries.Equal
)

func testSettings(t *testing.T) {
	t.Parallel()

	query := Settings()

	if query.Query == nil {
		t.Error("expected a query, got nothing")
	}
}

func testSettingsDelete(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if rowsAff, err := o.Delete(ctx, tx); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("should only have deleted one row, but affected:", rowsAff)
	}

	count, err := Settings().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testSettingsQueryDeleteAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if rowsAff, err := Settings().DeleteAll(ctx, tx); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("should only have deleted one row, but affected:", rowsAff)
	}

	count, err := Settings().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testSettingsSliceDeleteAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	slice := SettingSlice{o}

	if rowsAff, err := slice.DeleteAll(ctx, tx); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("should only have deleted one row, but affected:", rowsAff)
	}

	count, err := Settings().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testSettingsExists(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	e, err := SettingExists(ctx, tx, o.ID)
	if err != nil {
		t.Errorf("Unable to check if Setting exists: %s", err)
	}
	if !e {
		t.Errorf("Expected SettingExists to return true, but got false.")
	}
}

func testSettingsFind(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	settingFound, err := FindSetting(ctx, tx, o.ID)
	if err != nil {
		t.Error(err)
	}

	if settingFound == nil {
		t.Error("want a record, got nil")
	}
}

func testSettingsBind(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if err = Settings().Bind(ctx, tx, o); err != nil {
		t.Error(err)
	}
}

func testSettingsOne(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if x, err := Settings().One(ctx, tx); err != nil {
		t.Error(err)
	} else if x == nil {
		t.Error("expected to get a non nil record")
	}
}

func testSettingsAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	settingOne := &Setting{}
	settingTwo := &Setting{}
	if err = randomize.Struct(seed, settingOne, settingDBTypes, false, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}
	if err = randomize.Struct(seed, settingTwo, settingDBTypes, false, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = settingOne.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}
	if err = settingTwo.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	slice, err := Settings().All(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if len(slice) != 2 {
		t.Error("want 2 records, got:", len(slice))
	}
}

func testSettingsCount(t *testing.T) {
	t.Parallel()

	var err error
	seed := randomize.NewSeed()
	settingOne := &Setting{}
	settingTwo := &Setting{}
	if err = randomize.Struct(seed, settingOne, settingDBTypes, false, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}
	if err = randomize.Struct(seed, settingTwo, settingDBTypes, false, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = settingOne.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}
	if err = settingTwo.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	count, err := Settings().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 2 {
		t.Error("want 2 records, got:", count)
	}
}

func settingBeforeInsertHook(ctx context.Context, e boil.ContextExecutor, o *Setting) error {
	*o = Setting{}
	return nil
}

func settingAfterInsertHook(ctx context.Context, e boil.ContextExecutor, o *Setting) error {
	*o = Setting{}
	return nil
}

func settingAfterSelectHook(ctx context.Context, e boil.ContextExecutor, o *Setting) error {
	*o = Setting{}
	return nil
}

func settingBeforeUpdateHook(ctx context.Context, e boil.ContextExecutor, o *Setting) error {
	*o = Setting{}
	return nil
}

func settingAfterUpdateHook(ctx context.Context, e boil.ContextExecutor, o *Setting) error {
	*o = Setting{}
	return nil
}

func settingBeforeDeleteHook(ctx context.Context, e boil.ContextExecutor, o *Setting) error {
	*o = Setting{}
	return nil
}

func settingAfterDeleteHook(ctx context.Context, e boil.ContextExecutor, o *Setting) error {
	*o = Setting{}
	return nil
}

func settingBeforeUpsertHook(ctx context.Context, e boil.ContextExecutor, o *Setting) error {
	*o = Setting{}
	return nil
}

func settingAfterUpsertHook(ctx context.Context, e boil.ContextExecutor, o *Setting) error {
	*o = Setting{}
	return nil
}

func testSettingsHooks(t *testing.T) {
	t.Parallel()

	var err error

	ctx := context.Background()
	empty := &Setting{}
	o := &Setting{}

	seed := randomize.NewSeed()
	if err = randomize.Struct(seed, o, settingDBTypes, false); err != nil {
		t.Errorf("Unable to randomize Setting object: %s", err)
	}

	AddSettingHook(boil.BeforeInsertHook, settingBeforeInsertHook)
	if err = o.doBeforeInsertHooks(ctx, nil); err != nil {
		t.Errorf("Unable to execute doBeforeInsertHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected BeforeInsertHook function to empty object, but got: %#v", o)
	}
	settingBeforeInsertHooks = []SettingHook{}

	AddSettingHook(boil.AfterInsertHook, settingAfterInsertHook)
	if err = o.doAfterInsertHooks(ctx, nil); err != nil {
		t.Errorf("Unable to execute doAfterInsertHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected AfterInsertHook function to empty object, but got: %#v", o)
	}
	settingAfterInsertHooks = []SettingHook{}

	AddSettingHook(boil.AfterSelectHook, settingAfterSelectHook)
	if err = o.doAfterSelectHooks(ctx, nil); err != nil {
		t.Errorf("Unable to execute doAfterSelectHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected AfterSelectHook function to empty object, but got: %#v", o)
	}
	settingAfterSelectHooks = []SettingHook{}

	AddSettingHook(boil.BeforeUpdateHook, settingBeforeUpdateHook)
	if err = o.doBeforeUpdateHooks(ctx, nil); err != nil {
		t.Errorf("Unable to execute doBeforeUpdateHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected BeforeUpdateHook function to empty object, but got: %#v", o)
	}
	settingBeforeUpdateHooks = []SettingHook{}

	AddSettingHook(boil.AfterUpdateHook, settingAfterUpdateHook)
	if err = o.doAfterUpdateHooks(ctx, nil); err != nil {
		t.Errorf("Unable to execute doAfterUpdateHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected AfterUpdateHook function to empty object, but got: %#v", o)
	}
	settingAfterUpdateHooks = []SettingHook{}

	AddSettingHook(boil.BeforeDeleteHook, settingBeforeDeleteHook)
	if err = o.doBeforeDeleteHooks(ctx, nil); err != nil {
		t.Errorf("Unable to execute doBeforeDeleteHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected BeforeDeleteHook function to empty object, but got: %#v", o)
	}
	settingBeforeDeleteHooks = []SettingHook{}

	AddSettingHook(boil.AfterDeleteHook, settingAfterDeleteHook)
	if err = o.doAfterDeleteHooks(ctx, nil); err != nil {
		t.Errorf("Unable to execute doAfterDeleteHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected AfterDeleteHook function to empty object, but got: %#v", o)
	}
	settingAfterDeleteHooks = []SettingHook{}

	AddSettingHook(boil.BeforeUpsertHook, settingBeforeUpsertHook)
	if err = o.doBeforeUpsertHooks(ctx, nil); err != nil {
		t.Errorf("Unable to execute doBeforeUpsertHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected BeforeUpsertHook function to empty object, but got: %#v", o)
	}
	settingBeforeUpsertHooks = []SettingHook{}

	AddSettingHook(boil.AfterUpsertHook, settingAfterUpsertHook)
	if err = o.doAfterUpsertHooks(ctx, nil); err != nil {
		t.Errorf("Unable to execute doAfterUpsertHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected AfterUpsertHook function to empty object, but got: %#v", o)
	}
	settingAfterUpsertHooks = []SettingHook{}
}

func testSettingsInsert(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	count, err := Settings().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

func testSettingsInsertWhitelist(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Whitelist(settingColumnsWithoutDefault...)); err != nil {
		t.Error(err)
	}

	count, err := Settings().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

func testSettingToOneUserUsingUser(t *testing.T) {
	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()

	var local Setting
	var foreign User

	seed := randomize.NewSeed()
	if err := randomize.Struct(seed, &local, settingDBTypes, false, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}
	if err := randomize.Struct(seed, &foreign, userDBTypes, false, userColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize User struct: %s", err)
	}

	if err := foreign.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Fatal(err)
	}

	local.UserID = foreign.ID
	if err := local.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Fatal(err)
	}

	check, err := local.User().One(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}

	if check.ID != foreign.ID {
		t.Errorf("want: %v, got %v", foreign.ID, check.ID)
	}

	ranAfterSelectHook := false
	AddUserHook(boil.AfterSelectHook, func(ctx context.Context, e boil.ContextExecutor, o *User) error {
		ranAfterSelectHook = true
		return nil
	})

	slice := SettingSlice{&local}
	if err = local.L.LoadUser(ctx, tx, false, (*[]*Setting)(&slice), nil); err != nil {
		t.Fatal(err)
	}
	if local.R.User == nil {
		t.Error("struct should have been eager loaded")
	}

	local.R.User = nil
	if err = local.L.LoadUser(ctx, tx, true, &local, nil); err != nil {
		t.Fatal(err)
	}
	if local.R.User == nil {
		t.Error("struct should have been eager loaded")
	}

	if !ranAfterSelectHook {
		t.Error("failed to run AfterSelect hook for relationship")
	}
}

func testSettingToOneSetOpUserUsingUser(t *testing.T) {
	var err error

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()

	var a Setting
	var b, c User

	seed := randomize.NewSeed()
	if err = randomize.Struct(seed, &a, settingDBTypes, false, strmangle.SetComplement(settingPrimaryKeyColumns, settingColumnsWithoutDefault)...); err != nil {
		t.Fatal(err)
	}
	if err = randomize.Struct(seed, &b, userDBTypes, false, strmangle.SetComplement(userPrimaryKeyColumns, userColumnsWithoutDefault)...); err != nil {
		t.Fatal(err)
	}
	if err = randomize.Struct(seed, &c, userDBTypes, false, strmangle.SetComplement(userPrimaryKeyColumns, userColumnsWithoutDefault)...); err != nil {
		t.Fatal(err)
	}

	if err := a.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Fatal(err)
	}
	if err = b.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Fatal(err)
	}

	for i, x := range []*User{&b, &c} {
		err = a.SetUser(ctx, tx, i != 0, x)
		if err != nil {
			t.Fatal(err)
		}

		if a.R.User != x {
			t.Error("relationship struct not set to correct value")
		}

		if x.R.Settings[0] != &a {
			t.Error("failed to append to foreign relationship struct")
		}
		if a.UserID != x.ID {
			t.Error("foreign key was wrong value", a.UserID)
		}

		zero := reflect.Zero(reflect.TypeOf(a.UserID))
		reflect.Indirect(reflect.ValueOf(&a.UserID)).Set(zero)

		if err = a.Reload(ctx, tx); err != nil {
			t.Fatal("failed to reload", err)
		}

		if a.UserID != x.ID {
			t.Error("foreign key was wrong value", a.UserID, x.ID)
		}
	}
}

func testSettingsReload(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if err = o.Reload(ctx, tx); err != nil {
		t.Error(err)
	}
}

func testSettingsReloadAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	slice := SettingSlice{o}

	if err = slice.ReloadAll(ctx, tx); err != nil {
		t.Error(err)
	}
}

func testSettingsSelect(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	slice, err := Settings().All(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if len(slice) != 1 {
		t.Error("want one record, got:", len(slice))
	}
}

var (
	settingDBTypes = map[string]string{`ID`: `TEXT`, `CreatedAt`: `DATE`, `UpdatedAt`: `DATE`, `Data`: `BLOB`, `UserID`: `TEXT`}
	_              = bytes.MinRead
)

func testSettingsUpdate(t *testing.T) {
	t.Parallel()

	if 0 == len(settingPrimaryKeyColumns) {
		t.Skip("Skipping table with no primary key columns")
	}
	if len(settingAllColumns) == len(settingPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	count, err := Settings().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}

	if err = randomize.Struct(seed, o, settingDBTypes, true, settingPrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	if rowsAff, err := o.Update(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("should only affect one row but affected", rowsAff)
	}
}

func testSettingsSliceUpdateAll(t *testing.T) {
	t.Parallel()

	if len(settingAllColumns) == len(settingPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	o := &Setting{}
	if err = randomize.Struct(seed, o, settingDBTypes, true, settingColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	count, err := Settings().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}

	if err = randomize.Struct(seed, o, settingDBTypes, true, settingPrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	// Remove Primary keys and unique columns from what we plan to update
	var fields []string
	if strmangle.StringSliceMatch(settingAllColumns, settingPrimaryKeyColumns) {
		fields = settingAllColumns
	} else {
		fields = strmangle.SetComplement(
			settingAllColumns,
			settingPrimaryKeyColumns,
		)
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	typ := reflect.TypeOf(o).Elem()
	n := typ.NumField()

	updateMap := M{}
	for _, col := range fields {
		for i := 0; i < n; i++ {
			f := typ.Field(i)
			if f.Tag.Get("boil") == col {
				updateMap[col] = value.Field(i).Interface()
			}
		}
	}

	slice := SettingSlice{o}
	if rowsAff, err := slice.UpdateAll(ctx, tx, updateMap); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("wanted one record updated but got", rowsAff)
	}
}

func testSettingsUpsert(t *testing.T) {
	t.Parallel()
	if len(settingAllColumns) == len(settingPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	// Attempt the INSERT side of an UPSERT
	o := Setting{}
	if err = randomize.Struct(seed, &o, settingDBTypes, true); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Upsert(ctx, tx, false, nil, boil.Infer(), boil.Infer()); err != nil {
		t.Errorf("Unable to upsert Setting: %s", err)
	}

	count, err := Settings().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}
	if count != 1 {
		t.Error("want one record, got:", count)
	}

	// Attempt the UPDATE side of an UPSERT
	if err = randomize.Struct(seed, &o, settingDBTypes, false, settingPrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize Setting struct: %s", err)
	}

	if err = o.Upsert(ctx, tx, true, nil, boil.Infer(), boil.Infer()); err != nil {
		t.Errorf("Unable to upsert Setting: %s", err)
	}

	count, err = Settings().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}
	if count != 1 {
		t.Error("want one record, got:", count)
	}
}
