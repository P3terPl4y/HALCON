package migrations

import "goravel/app/facades"

type M20261001010000TrackingIndexes struct{}

func (*M20261001010000TrackingIndexes) Signature() string { return "20261001010000_tracking_indexes" }
func (*M20261001010000TrackingIndexes) Up() error {
	for _, sql := range []string{
		"CREATE INDEX IF NOT EXISTS idx_halcones_recipient ON halcones(recipient_id) WHERE recipient_id IS NOT NULL",
		"CREATE INDEX IF NOT EXISTS idx_assignments_active_user ON halcon_assignments(user_id, halcon_id) WHERE ended_at IS NULL",
		"CREATE INDEX IF NOT EXISTS idx_assignments_active_halcon ON halcon_assignments(halcon_id, assigned_at DESC) WHERE ended_at IS NULL",
	} {
		if _, err := facades.Schema().Orm().Query().Exec(sql); err != nil {
			return err
		}
	}
	return nil
}
func (*M20261001010000TrackingIndexes) Down() error {
	_, err := facades.Schema().Orm().Query().Exec("DROP INDEX IF EXISTS idx_halcones_recipient, idx_assignments_active_user, idx_assignments_active_halcon")
	return err
}
