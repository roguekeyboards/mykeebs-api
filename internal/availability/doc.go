// Package availability works out what in an owner's collection is free for
// a new build: what's been delivered, less what each keyboard's current
// build uses. It reads every one of the owner's builds whatever the
// caller may see, since a build the caller can't read still uses its
// parts.
package availability
