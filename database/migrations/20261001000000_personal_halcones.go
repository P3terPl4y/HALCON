package migrations

import (
	"goravel/app/facades"
)

type M20261001000000PersonalHalcones struct{}

func (*M20261001000000PersonalHalcones) Signature() string { return "20261001000000_personal_halcones" }
func (*M20261001000000PersonalHalcones) Up() error {
	// Goravel's migrator owns the transaction. Starting another transaction
	// here would commit the outer transaction prematurely in framework 1.18.
	query := facades.Schema().Orm().Query()
	for _, sql := range []string{
		"ALTER TABLE halcones ADD COLUMN IF NOT EXISTS owner_id bigint REFERENCES users(id) ON DELETE CASCADE, ADD COLUMN IF NOT EXISTS recipient_id bigint REFERENCES users(id) ON DELETE SET NULL",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_halcones_owner ON halcones(owner_id) WHERE owner_id IS NOT NULL",
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'halcones'::regclass AND conname = 'personal_recipient') THEN
				ALTER TABLE halcones ADD CONSTRAINT personal_recipient CHECK (recipient_id IS NULL OR (owner_id IS NOT NULL AND recipient_id <> owner_id));
			END IF;
		END $$`,
		"ALTER TABLE halcones ALTER COLUMN last_lat TYPE numeric(10,7), ALTER COLUMN last_lng TYPE numeric(10,7)",
	} {
		if _, err := query.Exec(sql); err != nil {
			return err
		}
	}
	return nil
}
func (*M20261001000000PersonalHalcones) Down() error {
	_, err := facades.Schema().Orm().Query().Exec("ALTER TABLE halcones DROP CONSTRAINT personal_recipient, DROP COLUMN recipient_id, DROP COLUMN owner_id")
	return err
}
