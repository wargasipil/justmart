package db

import (
	"reflect"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// registerUUIDDefault installs a before-create callback that gives every new row
// a Go-generated UUID for its string primary key when one isn't already set.
//
// On Postgres the schema declares `DEFAULT gen_random_uuid()`, so the database
// fills the id. SQLite has no UUID function, and the model tags still carry the
// `default:gen_random_uuid()` hint — which would make GORM OMIT the id from the
// INSERT and leave the NOT NULL primary key empty. Setting the value here
// (before gorm:create builds the statement) makes GORM include it instead.
//
// Integer primary keys (audit_log's AUTOINCREMENT) and already-set ids are left
// untouched, so composite-key join tables and caller-supplied ids are safe.
func registerUUIDDefault(gdb *gorm.DB) error {
	return gdb.Callback().
		Create().
		Before("gorm:create").
		Register("justmart:uuid_default", fillUUIDPrimaryKey)
}

func fillUUIDPrimaryKey(db *gorm.DB) {
	if db.Statement.Schema == nil {
		return
	}
	field := db.Statement.Schema.PrioritizedPrimaryField
	if field == nil || field.FieldType.Kind() != reflect.String {
		return
	}
	switch db.Statement.ReflectValue.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < db.Statement.ReflectValue.Len(); i++ {
			setUUIDIfEmpty(db, field, db.Statement.ReflectValue.Index(i))
		}
	case reflect.Struct:
		setUUIDIfEmpty(db, field, db.Statement.ReflectValue)
	}
}

func setUUIDIfEmpty(db *gorm.DB, field *schema.Field, rv reflect.Value) {
	if !rv.CanAddr() {
		return
	}
	if _, isZero := field.ValueOf(db.Statement.Context, rv); isZero {
		_ = field.Set(db.Statement.Context, rv, uuid.NewString())
	}
}
