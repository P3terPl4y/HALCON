package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/app/facades"
)

type M20260925175242CreateHalconTable struct{}

// Signature The unique signature for the migration.
func (r *M20260925175242CreateHalconTable) Signature() string {
	return "20260925175242_create_halcon_table"
}

// Up Run the migrations.
func (r *M20260925175242CreateHalconTable) Up() error {
	if !facades.Schema().HasTable("halcones") {
		return facades.Schema().Create("halcones", func(table schema.Blueprint) {
			table.TimestampTz("created_at").Nullable()
			table.TimestampTz("updated_at").Nullable()
			table.BigIncrements("id")
			table.String("name")
			table.String("token")
			table.Boolean("is_active").Default(false)
			table.Decimal("last_lat")
			table.Decimal("last_lng")
			table.TimestampTz("last_seen").Nullable()

			table.Unique("token").Name("idx_halcones_token")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20260925175242CreateHalconTable) Down() error {
	return facades.Schema().DropIfExists("halcones")
}
