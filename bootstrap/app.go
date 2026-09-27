package bootstrap

import (
	"github.com/goravel/framework/contracts/database/schema"
	contractsfoundation "github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/foundation"

	"goravel/app/facades"
	"goravel/app/models"
	"goravel/config"
)

func Boot() contractsfoundation.Application {
	return foundation.Setup().
		WithMigrations(Migrations).
		WithProviders(Providers).
		WithConfig(config.Boot).
		WithCallback(func() {
			facades.Schema().Extend(schema.Extension{
				Models: []any{

					&models.User{},
		&models.Halcon{},
		&models.HalconAssignment{},
				},
			})
		}).
		Create()
}
