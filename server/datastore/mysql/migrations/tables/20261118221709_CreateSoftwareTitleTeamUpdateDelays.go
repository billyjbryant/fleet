package tables

import (
	"database/sql"
	"fmt"
)

func init() {
	MigrationClient.AddMigration(Up_20261118221709, Down_20261118221709)
}

// Up_20261118221709 stores the per-app delay, in days, before a new Fleet-maintained app version is
// promoted. team_id 0 is Unassigned, so it has no foreign key to teams.
func Up_20261118221709(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS software_title_team_update_delays (
			team_id INT UNSIGNED NOT NULL,
			title_id INT UNSIGNED NOT NULL,
			update_delay_days SMALLINT UNSIGNED NOT NULL,
			updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (team_id, title_id),
			CONSTRAINT fk_update_delay_title FOREIGN KEY (title_id) REFERENCES software_titles (id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`); err != nil {
		return fmt.Errorf("creating software_title_team_update_delays: %w", err)
	}
	return nil
}

func Down_20261118221709(tx *sql.Tx) error {
	return nil
}
