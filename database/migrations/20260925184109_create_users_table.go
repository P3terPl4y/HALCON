package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/app/facades"
)

type M20260925184109CreateUsersTable struct{}

// Signature The unique signature for the migration.
func (r *M20260925184109CreateUsersTable) Signature() string {
	return "20260925184109_create_users_table"
}

// Up Run the migrations.
func (r *M20260925184109CreateUsersTable) Up() error {
	if !facades.Schema().HasTable("users") {
		return facades.Schema().Create("users", func(table schema.Blueprint) {
			table.TimestampTz("created_at").Nullable()
			table.TimestampTz("updated_at").Nullable()
			table.BigIncrements("id")
			table.String("name")
			table.String("email")
			table.String("phone")
			table.String("password")
			table.Boolean("status").Default(true)
			table.String("role").Default("user")

			table.Unique("email").Name("idx_users_email")
			table.Unique("phone").Name("idx_users_phone")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20260925184109CreateUsersTable) Down() error {
	return facades.Schema().DropIfExists("users")
}
