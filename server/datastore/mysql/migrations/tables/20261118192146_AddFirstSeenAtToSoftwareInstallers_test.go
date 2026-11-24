package tables

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUp_20261118192146(t *testing.T) {
	db := applyUpToPrev(t)

	scriptID := execNoErrLastID(t, db, `INSERT INTO script_contents (md5_checksum, contents) VALUES (UNHEX(MD5('echo')), 'echo')`)
	fmaID := execNoErrLastID(t, db, `INSERT INTO fleet_maintained_apps (name, slug, platform, unique_identifier) VALUES ('App', 'app/darwin', 'darwin', 'com.app')`)

	const insertInstaller = `INSERT INTO software_installers
		(filename, version, platform, install_script_content_id, uninstall_script_content_id, storage_id,
		 package_ids, patch_query, fleet_maintained_app_id, uploaded_at)
		VALUES (?, ?, 'darwin', ?, ?, ?, '', '', ?, '2026-01-02 03:04:05')`
	fmaInstallerID := execNoErrLastID(t, db, insertInstaller, "fma.pkg", "1.0", scriptID, scriptID, "fma-storage", fmaID)
	plainInstallerID := execNoErrLastID(t, db, insertInstaller, "plain.pkg", "1.0", scriptID, scriptID, "plain-storage", nil)

	applyNext(t, db)

	var firstSeen, uploaded time.Time
	require.NoError(t, db.QueryRow(`SELECT first_seen_at, uploaded_at FROM software_installers WHERE id = ?`, fmaInstallerID).Scan(&firstSeen, &uploaded))
	require.Equal(t, uploaded, firstSeen)

	var plainFirstSeen *time.Time
	require.NoError(t, db.QueryRow(`SELECT first_seen_at FROM software_installers WHERE id = ?`, plainInstallerID).Scan(&plainFirstSeen))
	require.NotNil(t, plainFirstSeen)
}
