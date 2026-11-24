package tables

import (
	"database/sql"
	"fmt"
)

func init() {
	MigrationClient.AddMigration(Up_20261118192146, Down_20261118192146)
}

func Up_20261118192146(tx *sql.Tx) error {
	if !columnExists(tx, "software_installers", "first_seen_at") {
		if _, err := tx.Exec(`ALTER TABLE software_installers
			ADD COLUMN first_seen_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			ALGORITHM=INSTANT`); err != nil {
			return fmt.Errorf("add first_seen_at to software_installers: %w", err)
		}
	}

	// Existing FMA versions have been around since they were uploaded.
	if _, err := tx.Exec(`UPDATE software_installers SET first_seen_at = uploaded_at WHERE fleet_maintained_app_id IS NOT NULL`); err != nil {
		return fmt.Errorf("backfill software_installers.first_seen_at: %w", err)
	}

	return nil
}

func Down_20261118192146(tx *sql.Tx) error {
	return nil
}
