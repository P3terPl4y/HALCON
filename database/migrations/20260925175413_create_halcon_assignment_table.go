package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/app/facades"
)

type M20260925175413CreateHalconAssignmentTable struct{}

// Signature The unique signature for the migration.
func (r *M20260925175413CreateHalconAssignmentTable) Signature() string {
	return "20260925175413_create_halcon_assignment_table"
}

// Up Run the migrations.
func (r *M20260925175413CreateHalconAssignmentTable) Up() error {
	if !facades.Schema().HasTable("halcon_assignments") {
		return facades.Schema().Create("halcon_assignments", func(table schema.Blueprint) {
			table.TimestampTz("created_at").Nullable()
			table.TimestampTz("updated_at").Nullable()
			table.BigIncrements("id")
			table.UnsignedBigInteger("halcon_id")
			table.UnsignedBigInteger("user_id")
			table.UnsignedBigInteger("moderator_id")
			table.String("package_id")
			table.TimestampTz("assigned_at")
			table.TimestampTz("ended_at").Nullable()

			table.Index("halcon_id").Name("idx_halcon_assignments_halcon_id")
			table.Index("moderator_id").Name("idx_halcon_assignments_moderator_id")
			table.Index("user_id").Name("idx_halcon_assignments_user_id")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20260925175413CreateHalconAssignmentTable) Down() error {
	return facades.Schema().DropIfExists("halcon_assignments")
}
