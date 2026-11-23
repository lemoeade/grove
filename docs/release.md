# Packaging and releases

This doc covers Grove's versioning rules, builds, DMG packaging, releases (GitHub Actions), auto-updates, signing,
notarization and CI. For everyday development, `go tool mygo build -skip-dmg -skip-notarize -sign -` is all you need;
see the "Building" section of the README.

## Versions

The version number always comes from `version` in `mygo.json`; public versions start at 0.1.0.

- `mygo build` writes it into Info.plist (`CFBundleShortVersionString` / `CFBundleVersion`) and compiles it into the
  binary with `-ldflags -X github.com/egoist/mygo.packageVersion=…`; at startup `main` sets
  `version.App = mygo.App.Version()`. `go run` and tests don't inject a version, so it shows as `dev`.
- The About window shows "Version x.y.z", with the mygo and Go versions it was built with below. On first launch or
  after an update, the status bar shows "Welcome to Grove x.y.z" / "Updated to Grove x.y.z" (recorded as `appVersion`
  in state.json, so the same version isn't announced twice). The first line of crash logs and `workbench.log` also
  records the version.
- `TestVersions` (`internal/version`) checks two things: the root `CHANGELOG.md` must have a section starting with
  `## v<version> `, and the mygo version compiled in must match the one pinned in go.mod.

Before 1.0, bump the patch version for fixes only (0.1.0 → 0.1.1) and the minor version for new features or behavior
changes (0.1.x → 0.2.0).

Everyday changes don't touch the version number. PRs that change behavior add an entry under `## Unreleased` at the
top of `CHANGELOG.md`, and `mygo.json` keeps the version of the last release.

Changelog sections are headed `## vX.Y.Z — YYYY-MM-DD`, with the unreleased section `## Unreleased` above them.

To release a new version:

1. Change `version` in `mygo.json`.
2. Rename `## Unreleased` in `CHANGELOG.md` to `## v<version> — <date>`, and add a new empty `## Unreleased` above it.
   This section is the "what changed" text in the update window (mygo writes it into the update manifest), so write it
   for users.
3. `GOTOOLCHAIN=auto go vet ./... && GOTOOLCHAIN=auto go test -count=1 ./... && GOTOOLCHAIN=auto go test -race -timeout 30m ./...`
   (`-race` takes 10–15 minutes, nearly all of it `internal/ui`, which is past go test's default 10-minute timeout)
4. Build as described below, and manually check in a real window what screenshots can't verify (menus, dialogs,
   vibrancy, drag and drop, terminal).
5. Commit (`chore(release): v<version>`), push to main, then tag that commit and push the tag:
   `git tag v<version> && git push origin v<version>`.
6. Once `release.yml` has created the draft Release and uploaded the files (ten minutes or so), look over the draft on
   the GitHub Releases page and click Publish. Only after publishing do installed copies of Grove see the new version,
   and only then does `install.sh` install it.

## Building

```bash
GOTOOLCHAIN=auto go tool mygo build -skip-dmg -skip-notarize -sign -
```

The output is `build/darwin-arm64/Grove.app` (about 30 MB), ad hoc signed, which opens on the machine that built it.
In `mygo.json` the app name is Grove, the bundle id is `io.github.lemoeade.grove`, the executable is
`Contents/MacOS/Grove`, and the DMG file name is `grove-<version>-arm64.dmg` (mygo uses the slug of the app name).
`cmd/workbench` and `WORKBENCH_*` in the source keep the project's old name; `AGENTS.md` explains why.
Copied to another Mac, it is blocked by Gatekeeper (see
"[Installing without an Apple Developer account](#installing-without-an-apple-developer-account)" below).

The terminal background process (termd) isn't packaged separately. Given the `termd` argument
(`Grove termd <socket> <log>`), the same executable hands off to the background process entry point on the very first
line of `main`, opens no window and doesn't appear in the Dock. So signing and notarization still only deal with this
one executable.

The app icon is `assets/icon.png` (1024×1024, `icon` in `mygo.json`, which mygo converts to `AppIcon.icns`). It is drawn
in code by `internal/appicon`: after changing the drawing code, run `go test ./internal/appicon -update` to regenerate
it; the About window uses the same drawing code. The icon isn't kept in `resources/`, because that directory is copied
verbatim into `Contents/Resources`, and putting the icon there would package it twice.

`resources/` holds the font (JetBrains Mono Nerd Font) and `licenses/` (full texts of third-party licenses, opened by
"View Full Licenses" in the About window).

## DMG

```bash
GOTOOLCHAIN=auto go tool mygo build -skip-notarize -sign -
```

This also produces `build/darwin-arm64/grove-<version>-arm64.dmg`, with a "drag to Applications" window layout.
mygo's steps: `hdiutil create` makes a writable image → `hdiutil attach -readwrite` mounts it and writes `.DS_Store`
(icon positions) → `detach` → convert to read-only ULMO. It converts with `diskutil image create from` when the system
has it, and falls back to `hdiutil convert` only otherwise (`hdiutil convert` fails on macOS 27).

Some machines block `attach` because of mount permissions, which is why the README's everyday build command has
`-skip-dmg`. In that case you can skip mounting and create a read-only image straight from a directory, though the
resulting image has no window layout:

```bash
mkdir -p dist/stage && ditto build/darwin-arm64/Grove.app dist/stage/Grove.app && ln -s /Applications dist/stage/Applications
hdiutil create -srcfolder dist/stage -volname Grove -fs HFS+ -format ULMO -ov dist/Grove-<version>-arm64.dmg
hdiutil verify dist/Grove-<version>-arm64.dmg
```

Newer systems warn that `hdiutil create -volname -format` is deprecated and suggest `diskutil image create`; the
command still works for now. A DMG made this way can be notarized as described below as well: first run
`xcrun notarytool submit … --wait`, then `xcrun stapler staple`.

## Releasing (GitHub Actions)

`.github/workflows/release.yml` runs when a `v*` tag is pushed:

1. **Check, create the draft** (ubuntu): the tag must equal `v` + `version` from `mygo.json`, and `CHANGELOG.md` must
   have a `## v<version>` section; if either doesn't match, it stops (creating nothing). Then
   `gh release create --draft --verify-tag --generate-notes` creates a draft Release (the notes on the Release page are
   generated by GitHub from PRs and can be edited in the draft; the update window uses the CHANGELOG section, and the
   two are unrelated).
2. **Build, upload** (`macos-latest`): `go vet`, `go test`, picks the signing identity based on secrets (next section),
   then

   ```bash
   go tool mygo build -skip-notarize -sign "$SIGN_IDENTITY" -upload
   ```

   Files uploaded to the draft: `grove-<version>-arm64.dmg`, `grove-<version>-darwin-arm64.tar.gz` (the update
   package), delta packages like `grove-<old version>-to-<version>-darwin-arm64.delta` (from previously released
   versions, bsdiff), `update-darwin-arm64.json` (the update manifest: version, the CHANGELOG section, each package's
   URL, size and Ed25519 signature), `install.sh` and `SHA256SUMS`.

The draft isn't published automatically: review it, then click Publish. Re-running the workflow replaces the files in
the draft.

Secrets to set (from the repo root, in your own terminal; never paste private keys, certificates or passwords into
chats, issues or logs):

```bash
gh secret set MYGO_UPDATER_PRIVATE_KEY < "$HOME/Library/Application Support/mygo/update-keys/mygo-update.key"
# optional: a self-signed certificate (see the next section)
base64 -i grove-sign.p12 | gh secret set MACOS_CERTIFICATE
gh secret set MACOS_CERTIFICATE_PASSWORD
gh secret set MACOS_SIGN_IDENTITY --body "Grove Self-Signed"
```

## Auto-updates

Grove uses mygo's updater (`updates` in `mygo.json`, plugin `plugins/updater/native`, a native window):

- **Keys**: `go tool mygo keygen` generates an Ed25519 key pair, written by default to
  `~/Library/Application Support/mygo/update-keys/`. The public key (the contents of `mygo-update.pub`) goes in
  `updates.publicKey` in `mygo.json` and is compiled into the app; the private key (`mygo-update.key`) never goes into
  the repo: store it in a password manager, then set it as the secret `MYGO_UPDATER_PRIVATE_KEY`. **If the private key
  is lost, installed copies of Grove can never receive updates again** (they only accept packages signed with this
  key); users would have to reinstall, via install.sh, a version with a new public key. If the private key leaks,
  anyone can push "updates" to Grove users.
- **Checking**: release builds check
  `https://github.com/lemoeade/grove/releases/latest/download/update-darwin-arm64.json` once a day in the background
  (only published Releases count, not drafts or pre-releases). When there is a new version, a window shows the
  CHANGELOG section, and the user picks Install, Skip This Version or Remind Me Later. Grove → "Check for Updates…" in
  the menu (also in the Command Palette) checks at any time. Settings → Update has "Automatically check for updates",
  "Automatically download and install updates" (takes effect on next launch), the current version and the time of the
  last check. These two toggles are stored in mygo's `updater.json` (`~/Library/Application Support/Grove/`), not in
  state.json.
- **Installing**: download (a delta package first, when available) → verify the Ed25519 signature → replace
  `/Applications/Grove.app` (or wherever the user installed it) → relaunch. Update packages are downloaded curl-style,
  without the quarantine attribute, so they don't go through Gatekeeper: even un-notarized builds update themselves all
  the way.
- **When it doesn't check**: dev builds from `go run` and `mygo dev` (the Settings page says "this build can't update
  itself", and so does the window when you click the menu item); instances with `WORKBENCH_STATE` set don't check in
  the background (so no window steals the front during memory measurement or smoke tests), though a manual "Check for
  Updates…" still works.
- Code: `internal/app/updates.go` (the UI only goes through the `app.Updates` hooks, which are nil in tests),
  `setupUpdates` in `cmd/workbench/main.go`, and the "Update" page in `internal/ui/settings.go`.

## Installing without an Apple Developer account

Without an account you can't sign with Developer ID or notarize. That is how Grove ships today:

- **install.sh is the main route** (the one-liner in the README):

  ```bash
  curl -fsSL https://github.com/lemoeade/grove/releases/latest/download/install.sh | sh
  ```

  It reads the latest published update manifest, downloads the tar.gz, verifies the Ed25519 signature, installs it in
  `/Applications` (or `~/Applications` if it can't write there) and opens it. Files downloaded with curl have no
  quarantine attribute, so Gatekeeper doesn't block them and users don't have to click any "Open Anyway".
  `sh install.sh --uninstall` uninstalls (settings and data are kept).
- **DMG**: a DMG downloaded in a browser carries the quarantine attribute, and Gatekeeper blocks it ("developer cannot
  be verified" or "is damaged"). Since macOS 15, right-click → Open no longer gets around it. Either drag it into
  Applications, double-click once to get blocked, then go to System Settings → Privacy & Security and click "Open
  Anyway" at the bottom; or run `xattr -dr com.apple.quarantine /Applications/Grove.app` in Terminal. Only the first
  time needs this; later auto-updates never go through Gatekeeper.

### Signing identity: ad hoc or a self-signed certificate

- **ad hoc** (`-sign -`, when no certificate secrets are set): installs, runs and auto-updates. But the "designated
  requirement" of an ad hoc signature is the executable's hash, which differs in every version: to the system every
  update is a new app, so permissions already granted (Accessibility, Full Disk Access, the prompts for folders like
  Documents…) have to be granted again, and items stored in the keychain may ask again too.
- **A self-signed "Code Signing" certificate** (recommended): the designated requirement is pinned to the certificate
  (`certificate leaf = H"…"`), the same in every version, so permissions only need granting once. No account is needed,
  and it doesn't matter that the system doesn't trust the certificate (codesign signs anyway, and `find-identity`
  flagging it `CSSMERR_TP_NOT_TRUSTED` is normal). It doesn't get Gatekeeper to let the app through; first installs work
  the same as with ad hoc.

  Use macOS's own `/usr/bin/openssl` (LibreSSL; Homebrew's OpenSSL 3 needs `-legacy` when exporting the .p12, or
  `security import` won't recognize it):

  ```bash
  cat > grove-sign.cnf <<'EOF'
  [req]
  distinguished_name=dn
  x509_extensions=ext
  prompt=no
  [dn]
  CN=Grove Self-Signed
  [ext]
  basicConstraints=critical,CA:false
  keyUsage=critical,digitalSignature
  extendedKeyUsage=critical,codeSigning
  EOF
  /usr/bin/openssl req -x509 -newkey rsa:2048 -nodes -keyout grove-sign.key -out grove-sign.crt -days 7300 -config grove-sign.cnf
  /usr/bin/openssl pkcs12 -export -inkey grove-sign.key -in grove-sign.crt -out grove-sign.p12   # set a password when prompted
  ```

  Set `MACOS_CERTIFICATE` / `MACOS_CERTIFICATE_PASSWORD` / `MACOS_SIGN_IDENTITY` as in the previous section, then store
  `grove-sign.p12` and its password in a password manager and delete the local copies of `grove-sign.key` and the
  .p12. Changing the certificate means changing the signing identity, and users have to grant permissions again, so
  don't change it lightly. The one update that switches from ad hoc to self-signed also makes users grant them again.

When signing with a real identity (self-signed or Developer ID), mygo adds the hardened runtime and a secure timestamp
to the .app and the code in `Contents/Resources`. The hardened runtime turns on library validation by default: only
libraries signed with the same Team ID may load, and a self-signed certificate has no Team ID, so purego fails to load
`libghostty-vt.dylib` (the terminal shows "Failed to start", with "mapping process and mapped file (non-platform) have
different Team IDs"). That's why `macos.entitlements` in `mygo.json` points to `assets/entitlements.plist`, which
contains `com.apple.security.cs.disable-library-validation`. A local build signed with a self-signed certificate +
hardened runtime + this entitlement was verified: the terminal starts a shell and the single-instance lock works;
without the entitlement the terminal doesn't start. Ad hoc signatures don't get the hardened runtime, so the
entitlement has no effect on them.

## Signing and notarization (once there is an Apple Developer account)

This needs a paid Apple Developer account. The steps below haven't been verified in practice; they are put together
from mygo's code (`cmd/mygo/dmg.go`, `bundle.go`) and Apple's process.

1. **Certificate**: install a "Developer ID Application: Name (TEAMID)" certificate in the keychain and confirm with
   `security find-identity -v -p codesigning`.
2. **Notarization credentials**: store them in the keychain (once only; the password is an app-specific password
   generated at appleid.apple.com):

   ```bash
   xcrun notarytool store-credentials grove-notary --apple-id YourAppleID --team-id TEAMID
   ```

3. **mygo.json**: add these fields under `macos` (the signing identity can also stay out of the file and be passed at
   build time with `-sign "Developer ID Application: …"`):

   ```json
   "signingIdentity": "Developer ID Application: Name (TEAMID)",
   "notarize": { "keychainProfile": "grove-notary" }
   ```

   Once `notarize` is configured, builds with an ad hoc signature (`-sign -`) must pass `-skip-notarize`, or mygo
   reports an error. The commands in the README and CI already include it.
4. **Build**: `GOTOOLCHAIN=auto go tool mygo build`. When signing with Developer ID, mygo adds the hardened runtime and
   a secure timestamp, also signs the executables and dylibs in `Contents/Resources` (the libghostty-vt mygo puts
   there), produces the DMG, then runs `notarytool submit --wait`. Once it passes, `stapler staple` attaches the
   notarization ticket to the DMG and the .app.
5. **Verify**:

   ```bash
   codesign -dv --verbose=4 build/darwin-arm64/Grove.app
   spctl -a -vv -t install build/darwin-arm64/grove-<version>-arm64.dmg
   xcrun stapler validate build/darwin-arm64/grove-<version>-arm64.dmg
   ```

   If verification fails, view the log with `xcrun notarytool log <submission ID> --keychain-profile grove-notary`
   (mygo prints this command when it fails).

Disabling library validation in `assets/entitlements.plist` was added for the self-signed certificate; with Developer
ID the libraries and the app share a Team ID, so it can be removed (notarization allows this entitlement, so keeping it
also passes). To switch CI to Developer ID: follow mygo's `docs/github-actions.md` to import the Developer ID .p12 and
notarization credentials, and drop `-skip-notarize` from the build in `release.yml`. After changing the signing
identity, users have to grant permissions again; the Ed25519 key for update packages doesn't need to change.

## Opening files and single instance

`macos.infoPlist` in `mygo.json` adds `CFBundleDocumentTypes`: folders (`public.folder`) and any file (`public.item`),
with `LSHandlerRank` set to `Alternate`: Grove shows up in Finder's "Open With" and files can be dragged onto its Dock
icon, but Grove doesn't become the default app. The system may take a moment, or need the .app moved into
Applications, before it recognizes these types.

Single instance is implemented by `mygo.App.RequestSingleInstanceLock()`, which creates a unix socket keyed by the app
name and identifier. A second `Grove <path>` process hands its arguments and working directory to the running instance
and exits; `open -a Grove <path>` and files dragged onto the Dock icon are handled through `OnOpenFile`. When measuring
memory or testing, setting `WORKBENCH_STATE` skips the single-instance lock by default, so arguments don't go to the
instance you use every day. Add `WORKBENCH_SINGLE=1` only when testing single instance itself, and first check with
`pgrep -fl Grove` that no other instance is running.

## CI

For the release workflow `release.yml`, see "[Releasing (GitHub Actions)](#releasing-github-actions)" above. The
everyday `.github/workflows/ci.yml` runs on `macos-latest`:

- On every push to main and every PR, it runs gofmt, `go vet ./...`, `go test -count=1 ./...` and
  `mygo build -skip-dmg -skip-notarize -sign -`, zips the .app and keeps it as a build artifact for 14 days.
- `go test -race -timeout 30m ./internal/...` takes longer (`internal/ui` alone is over ten minutes), so it only runs when triggered manually with `race` checked, or once a
  week on Monday.

`GOTOOLCHAIN=auto` is set in env (go.mod requires 1.27.3); mygo uses the version pinned in go.mod, and CI must not run
`go get -u`. If UI tests and terminal tests are flaky on GitHub runners, check environment differences first: runners
have no login session, and the shell is `/bin/zsh`. CI doesn't set `WORKBENCH_TEST_WS`, so the read-only tests against
a real workspace are skipped.

`.github/workflows/claude.yml`: when the repo's owner, a member or a collaborator writes `@claude` in an issue or PR
comment, Claude runs on `macos-latest`. It uses the Claude Agent SDK (`.github/claude/agent.mjs`, versions pinned by
the `package-lock.json` in the same directory), not `claude-code-action`: the latter is billed as Claude Code, and the
console's free credits can't be used for it. Claude can read and write files and run any command (Bash isn't
restricted per command: it can `go run` arbitrary code, so finer restrictions block nothing and only waste turns), but
that step gets no GitHub token and can't push code. Pushing and opening PRs is done by `finish.sh`: on an issue it
creates a new branch `claude/issue-<number>-<time>` and opens a PR; on a PR from the same repo it pushes to the PR's
branch; PRs from forks aren't handled. If the run doesn't finish (it hits a limit or errors out), the half-done changes
are still pushed to a branch (on a PR, a separate `claude/wip-pr-<number>-<time>`), but no PR is opened, and the comment
links to the changes. If "Allow GitHub Actions to create and approve pull requests" isn't enabled in the repo settings,
it can't open a PR, and the comment gives a link to open one by hand.
The Anthropic API key must be saved as the secret `ANTHROPIC_API_KEY` (given only to the Claude step; the GitHub token
is given only to the prepare and finish steps, with `persist-credentials: false`). These events only read the workflow
file and scripts from the default branch, so changes take effect only after merging to main. Each run is capped at 100
turns, about $5 (`maxTurns`, `maxBudgetUsd` in `agent.mjs`) and 60 minutes. The repo is public, so the run log only
prints tool names; Claude's replies are posted back as issue / PR comments.
Claude runs `go test`, which means executing the PR's code, so don't `@claude` on PRs from people you don't know.

## Licenses

- Full texts of third-party licenses are in `resources/licenses/` (listed in `README.txt`) and can be opened from the
  About window.
- `ghostty.txt` contains the provenance note (the Ghostty commit libghostty-vt comes from) and the full LICENSE text at
  that commit (MIT requires including the original). When mygo moves to a different Ghostty commit
  (`plugins/terminal/mygo-plugin.json`), check it against the LICENSE at the new commit.
- Grove is under the MIT license (`LICENSE` at the root). To put copyright information in Info.plist, add a
  `copyright` field to `mygo.json`, which maps to `NSHumanReadableCopyright`.
