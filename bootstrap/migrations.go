package bootstrap

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/database/migrations"
)

func Migrations() []schema.Migration {
	return []schema.Migration{
		&migrations.M20260928184647CreateUserTable{},
		&migrations.M20260928184743CreateHalconTable{},
		&migrations.M20260928185003CreateHalconAssignmentTable{},
		&migrations.M20260928185035CreateHalconTable{},
		&migrations.M20261001000000PersonalHalcones{},
	}
}
