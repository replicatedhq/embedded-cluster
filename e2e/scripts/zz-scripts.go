// Package scripts embeds all shell scripts we use for testing.
// this file is named zz_ so it is the last file to show up
// in the editor.
package scripts

import "embed"

// FS is the embedded filesystem.
//
//go:embed *
var FS embed.FS
