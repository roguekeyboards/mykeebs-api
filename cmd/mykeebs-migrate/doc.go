// Command mykeebs-migrate dumps a user's profile and every entity they own
// (keyboards, switches, EC parts, keycap sets, builds), with every associated
// S3 image, to a local directory, and restores such a dump into another
// environment through the same public REST API.
//
// It exists to take a complete, verifiable offline copy before a
// backwards-incompatible data-model change, and doubles as a way to move a
// user's data between environments (dev to prod, either direction) and to
// export someone else's account for a data request (dump --impersonate).
//
// # Why cmd/
//
// The repo convention (see CLAUDE.md) is that the Lambda *function* entrypoint
// lives at functions/api/, deliberately not cmd/api/, because functions/api/
// signals "this is a Lambda function" in an AWS-specific codebase. That
// reasoning is specific to the deployed service. mykeebs-migrate is a standalone
// operational CLI run from a developer's machine, which is exactly what
// golang-standards/project-layout's cmd/<name>/ is for, and what internal/ and
// test/ in this repo already follow. So this one command does live under cmd/.
//
// # Subcommands
//
//	mykeebs-migrate login    - obtain an access token via browser OAuth (Discord)
//	mykeebs-migrate dump      - download the profile + every entity + image into --out
//	                         (--impersonate: as the user a Stytch dashboard impersonation opens)
//	mykeebs-migrate restore   - recreate a dump's entities + images in --base-url's environment
//	mykeebs-migrate verify    - check a restore against the dump (fields + image hashes)
//
// # Images
//
// dump converts JPEG and PNG images to WebP and scales any image over 2560px
// down, with the settings mykeebs-web uses on upload, so a restore leaves only
// WebP behind. The dump's files, manifests and hashes describe the converted
// bytes, which restore and verify then check against. An image the API would
// refuse on restore - another format, an animated image, or one still over
// the API's size cap after conversion - fails the dump.
//
// # What it does not do
//
// It never touches lookups, which are a static catalog built into the API;
// dump captures lookups/lookups.json for reference and diffing only, and
// restore ignores it.
//
// It never renames a profile. Restoring into an account with no profile
// creates one with the dumped username, and fails before writing anything if
// another account already has that username. Restoring into an account that
// has a profile replaces its fields, and its avatar if the dump has one, but
// keeps its username, which is what moving data between two accounts in one
// environment needs.
//
// It uses no AWS credentials. Image bytes move over signed URLs the REST
// API mints, so it works identically against any environment the caller can
// reach.
package main
