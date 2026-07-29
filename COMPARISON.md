# go-via vs go-via2 — comparison and fork assessment

Date of analysis: 2026-07-28

Both repos are by [maxiepax](https://github.com/maxiepax). They implement a
replacement for VMware's Imaging Appliance (VIA): a single Go binary that acts as
DHCP server, TFTP/HTTP boot server, `ks.cfg` (kickstart) generator and REST API,
for automated ESXi bare-metal deployment.

---

## TL;DR

There are **three** codebases, not two. The one that matters is not in either
repo's default branch.

| | `go-via` `main` | `go-via` `dev` | `go-via2` `main` |
|---|---|---|---|
| Last commit | 2024-10-30 | **2026-03-07** | 2025-07-22 |
| Commits | 711 | 711 + 16 | 8 |
| Go LOC (excl. generated docs) | ~5,300 | ~4,840 | ~3,585 |
| Compiles? | ✅ yes | ✅ yes | ❌ **no** |
| Frontend | Angular 11 + Clarity (complete) | Next.js 15 / React 19 (boilerplate) | none |
| Boot method | TFTP (PXE) only | TFTP + **HTTP/HTTPS UEFI boot** | HTTP only (unwired) |
| IP model | DHCP pools + leases | static host reservations | pools (broken) |
| Post-config (govmomi) | ✅ full | ⚠️ present, disabled | ❌ removed |
| License | GPL-3.0 | GPL-3.0 | **none** |
| Stars / forks / issues | 26 / 5 / 5 open | — | 0 / 0 / 0 |

**Recommendation: fork `go-via` and base your work on the `dev` branch.**
Do not fork `go-via2`.

---

## 1. `go-via` (main) — the shipping product

The mature line. 711 commits, 5 years old, tagged releases through `v0.4.5`,
published Docker images, GoReleaser + GitHub Actions.

Architecture:

```
main.go        → config, DB automigrate, seeds, gin router, TLS bootstrap
dhcp.go        → raw-socket DHCPv4 server (gopacket), per-interface goroutine
tftpd.go       → TFTP server; hijacks mboot.efi/crypto64.efi/boot.cfg per image
serve.go       → interface helpers
api/           → REST handlers (pools, addresses, options, device_classes,
                 groups, images, users, ks, postconfig)
models/        → GORM models + DHCP option encoding
crypto/        → self-signed CA + server cert generation
secrets/       → AES-256-GCM at-rest encryption for ESXi root passwords
websockets/    → live log streaming to the UI
web/           → Angular 11 + VMware Clarity SPA, embedded via statik
```

What it does well:

- **DHCP option layering** — options resolve global → pool → address →
  device-class with a priority `Level()` (`models/option.go`). Genuinely nice
  design.
- **Image hijacking** — `mboot.efi` / `crypto64.efi` / `boot.cfg` are served from
  inside the specific ESXi ISO the host is assigned, so you can never boot a
  mismatched loader. Handles both `EFI/BOOT/BOOTX64.EFI` and `BOOTAA64.EFI`
  (x86 + ARM/SmartNIC).
- **DHCP relay / IP-helper support** — the binary doesn't need to sit on the
  provisioning VLAN.
- **Post-config via govmomi** (`api/postconfig.go`, 644 lines) — after install it
  connects to the fresh ESXi host and configures NTP, syslog, SSH, certificates,
  VMFS, and the VCF 4.x/5.x prerequisites. This is the single biggest piece of
  domain value in the project and it exists nowhere else.
- **Progress tracking** — every stage (mboot 10%, crypto64 12%, boot.cfg 15%,
  kickstart 50%, …) writes progress to the DB and streams over websocket.

Weak points:

- Go 1.15 in `go.mod`, gin 1.7.3, Angular 11 (EOL 2022), `statik` (obsoleted by
  `go:embed` in Go 1.16).
- **`govulncheck`: 28 reachable vulnerabilities** across gin, gin-contrib/cors
  and the stdlib.
- **Zero tests.** No `*_test.go` anywhere in any of the three trees.
- CI runs only on tag push (GoReleaser). No build/test on PR.
- Auth is HTTP Basic revalidated per request with bcrypt — no sessions, no rate
  limiting, no lockout. Default `admin` / `VMware1!` seeded on first run.
- `ks.cfg` is served **unauthenticated**, authorised only by source IP, and its
  body contains the **decrypted ESXi root password**. Anyone who can spoof or
  occupy the host's IP on the provisioning VLAN gets the root password. This is
  arguably inherent to kickstart, but it deserves at minimum a one-shot token in
  the boot.cfg URL.
- `secrets.Decrypt` calls `panic()` on any decrypt failure — a corrupt DB field
  takes down the whole daemon.

## 2. `go-via2` — an abandoned rewrite

Started 2025-02-06, 8 commits, last touched 2025-07-22. `README.md` reads, in
full: *"This is a work in progress, currently working on the API, UI will follow.
Do not use this code!"*

It is a port of `go-via` to modern deps (Go 1.21, gin 1.10, gorm 1.25) with
`Address` renamed to `Host`, TFTP dropped in favour of an HTTP file handler
(`api/filehandler.go`), and `postconfig` deleted.

It is not usable, and the problems are structural, not cosmetic:

- **`go.sum` is not committed** → `go build` fails immediately on a clean clone.
- After `go mod tidy` it still fails: `main.go:19: "github.com/davecgh/go-spew/spew" imported and not used`.
- **`AutoMigrate` omits `Pool`, `Option` and `DeviceClass`** while `dhcpd` queries
  all three. Even once it compiles, DHCP hits missing tables at runtime.
- **Routes for pools, addresses, options, device_classes and users are absent**
  from `main.go`. There is no way to create the pool the DHCP server requires.
- DHCP option 66 is commented out and option 67 is the bare string `"mboot.efi"`,
  but **no TFTP server exists** — so a PXE client is told to TFTP a file from a
  host that isn't listening. The HTTP handler at `/esx/*` is never advertised
  over DHCP. It cannot complete a boot.
- **No LICENSE file.** GitHub reports no license → default copyright, all rights
  reserved. Forking it is legally murkier than forking the GPL-3.0 `go-via`.
- Debug scaffolding (`spew.Dump`) left in production paths.

`bootCfg()` also returns `fmt.Errorf("could not build boot.cfg")` on the success
path — harmless only because every caller discards the error.

## 3. `go-via` `dev` — where the project actually went

**This is the finding that changes the answer.** The `dev` branch is 16 commits
ahead of `main`, and its last commit is 2026-03-07 — five months ago, and eight
months *newer* than anything in `go-via2`. The author appears to have abandoned
the separate-repo rewrite and folded the same ideas back into `go-via`.

It carries everything `go-via2` was reaching for, plus more:

- `Address` → `Host` rename applied across the whole app.
- Pools removed; hosts are matched by MAC via `findHostByMAC()` and given a fixed
  reservation. (Pool code is commented out rather than deleted — see caveats.)
- `dhcp.go` → `dhcpd/` package; `uefi/` package added.
- **Working UEFI HTTP boot**: `dhcpd/dhcpd.go` inspects the client's vendor class
  (`HTTPClient:Arch:00016` / `:00011`, seeded as device classes alongside the
  existing `PXEClient:Arch:*`) and picks a boot method, then serves option 67 as a
  full URL — `http://<ip>/esx/mboot.efi`, `https://<ip>/esx/mboot.efi`, or the
  plain `mboot.efi` TFTP filename for legacy PXE. A second gin engine listens on
  :80 for boot only, while the API stays on :8443. TFTP is retained as a
  fallback. This is the correct design and it is *implemented*, not stubbed.
- Angular app deleted; `ui/` contains a Next.js 15 / React 19 / Tailwind 4 /
  Biome scaffold with `login`, `pool` and `groups` pages.
- `example-scripts/` — curl-based API examples and a build/wipe QA harness.
- `--forceunsupportedinstall` wired to the `AllowLegacyCPU` group option.
- Go 1.23/1.24 toolchain, `golang.org/x/crypto` bumped to v0.40.0.

Caveats — it is a work-in-progress branch, not a release:

- Large blocks of pool logic are commented out rather than removed; `models/pool.go`
  and `api/pools.go` are gone but `example-scripts/pools/` and `ui/src/app/pool/`
  still reference them.
- `api/login.go` is a single commented-out function (a session-auth attempt that
  was never finished — it references a `sessions` package that was never imported).
- `postconfig` still compiles but `go ProvisioningWorker(...)` is commented out in
  `api/ks.go`, so post-install configuration no longer runs.
- `ui/` is not built into the binary — `main.go` still serves the old Angular
  bundle from `statik`. The Next.js app is disconnected.
- `spew.Dump(filepath)` on every UEFI file request.
- Same 28 `govulncheck` findings as `main` (gin and gin-contrib/cors were never
  bumped).
- No tests, no CI beyond tag-triggered GoReleaser.

---

## Fork recommendation

**Fork `maxiepax/go-via`, branch from `dev`, keep GPL-3.0.**

Rationale: `dev` is the newest code, it compiles, it has the HTTP-boot work that
`go-via2` only gestured at, and it inherits five years of history, the release
pipeline, the GPL-3.0 license and the 26-star / 5-fork community. `go-via2` has
nothing `dev` lacks and several things `dev` has fixed.

Two things to do at fork time regardless:

1. **Cherry-pick, don't discard, `main`.** `dev` disabled `ProvisioningWorker`
   and deleted pools. Decide deliberately whether you want them back; `main` is
   the reference implementation for both.
2. **GPL-3.0 is viral.** Your fork and anything you link into it must stay
   GPL-3.0 and publish source. Fine for an internal or open tool; a blocker if
   this is meant to ship inside a closed product. Worth confirming before you
   invest.

## Proposed roadmap for the fork

Ordered so that each phase leaves the tree in a better state than it found it.

### Phase 0 — make it trustworthy — **done**

All of the following has landed on this fork; `fork-baseline` tags the
upstream `dev` commit it started from.

- ✅ Forked from `dev` with full upstream history preserved.
- ✅ Dependencies upgraded (gin 1.7.3 → 1.12.0, cors 1.3.1 → 1.7.7, gorm
  1.20 → 1.31, govmomi 0.24 → 0.55) and the toolchain pinned to Go 1.25.
  **`govulncheck`: 28 reachable vulnerabilities → 0.**
- ✅ `statik` replaced with `go:embed`, dropping a 22MB generated blob that
  held an Angular bundle calling API endpoints that no longer exist.
- ✅ CI on every push and PR: gofmt, build, vet, race tests, golangci-lint,
  govulncheck, and the frontend lint and build.
- ✅ Dead code removed — `api/login.go`, the commented pool blocks (`dhcpd`
  drops 941 → 689 lines), the duplicate UEFI handlers, the `spew.Dump` calls.
- ✅ `panic()` replaced with returned errors in `secrets`, plus a missing
  length check that crashed the daemon on an empty stored password.
- ✅ **51 golangci-lint findings → 0**, including several real bugs (see below).
- ✅ Release pipeline repaired — it could not have worked: Go 1.15 against a
  Go 1.25 module, goreleaser flags removed in v2, and a frontend build step
  with no Node toolchain.

Bugs found and fixed along the way, none of which were style issues:

| Where | Bug |
|---|---|
| `dhcpd` | Empty `if` body around the group lookup. On failure no option 67 was sent and the host silently never booted. |
| `dhcpd` | Boot-method chain had no fallback — an unset method failed the same silent way. |
| `dhcpd` | Errors from `c.WriteTo` (which sends the DHCP reply) and both `AddOptions` calls discarded. |
| `api/image.go` | Failed `os.Open` only warned, then dereferenced the nil file. |
| `api/image.go` | `log.Fatal` in three places inside an HTTP handler — an upload error killed the daemon. |
| `crypto` | `pem.Encode` and `Close` errors dropped when writing the CA cert, CA key and server keypair; `rsa.GenerateKey`'s error dropped into `_`. |
| `secrets` | `Decrypt` sliced `enc[:nonceSize]` unguarded — an empty password field panicked. |
| four files | Unchecked `json.Unmarshal` of the group options blob silently treated every option as false. |
| `main.go` | UEFI HTTP boot listener started with its error dropped; it could die silently. |
| `ui/` | `lucide-react` imported but absent from `package.json` — `next build` failed outright. |
| `.goreleaser.yml` | No ldflags, so every released binary reported version `dev`, commit `none`. |

Known gap this surfaced: **DHCP DECLINE is not handled.** The old
implementation depended on pools and cannot be restored as-is; it needs
rewriting against the host reservation model.

### Phase 1 — tests — **mostly done**

| package | coverage |
|---|---|
| `webui` | 100% |
| `secrets` | 80% |
| `models` | 78% |
| `dhcpd` | 56% |
| `api` | 6% (kickstart only) |

Done:

- ✅ `models.Option.ToDHCPOption()` — every option type, signed time offsets,
  the merge flag, malformed input, unsupported opcodes, `Level()` precedence.
- ✅ `dhcpd` address helpers — `NetmaskToCIDR`, `NetworkAddress`,
  `BroadcastAddress`, including non-contiguous masks. These were correct.
- ✅ `dhcpd` option generation and packet handling — DISCOVER/REQUEST/dispatch,
  boot method to option 67, header construction and broadcast-vs-unicast.
- ✅ `ks.cfg` rendering — networking, group options, boot disk, VLAN, syslog,
  both override paths, the reimage flag, and the password round trip. Correct
  as written; no defects found.

Four more real bugs surfaced, all found by a test written before the fix:

| Where | Bug |
|---|---|
| `models` | Subnet mask, broadcast and solicit options encoded as **16 bytes** — `net.ParseIP`'s IPv4-in-IPv6 form — instead of the 4 RFC 2132 defines. |
| `models` | An unparseable address returned a zero-length option and no error. |
| `dhcpd` | `AddOptions` **panicked** on a group with a valid netmask and a blank gateway (`slice bounds out of range [:3] with capacity 0`), on the DHCP goroutine — taking down the daemon. |
| `dhcpd` | REQUEST ACKed **any** address the client asked for, never comparing it to the reservation. The pool-based code checked this and NAK'd; the check was lost when pools were removed. |

Still to do:

- `boot.cfg` rewriting — the regex surgery in `uefi/uefi.go` is fragile and
  ESXi-version-dependent; pin its behaviour per ISO layout. This needs ISO
  fixtures, so it is the one piece not yet covered.
- The rest of the `api` package — handlers for hosts, groups, images, users.

### Phase 2 — finish what `dev` started

- Wire `ui/` into the binary via `go:embed` and delete the Angular remnants.
  The Next.js scaffold needs the actual screens: hosts, groups, images, live log,
  progress. This is the largest single chunk of work in the roadmap.
- Real authentication: session cookies or JWT, bcrypt already in place, plus
  rate limiting and forced change of the default `admin`/`VMware1!` on first
  login. Basic-auth-per-request is not viable for a browser SPA anyway.
- Decide the pools question. Static-reservation-only (dev's direction) is simpler
  and safer for an imaging appliance; dynamic pools are what the existing users
  have configured. If you drop pools, ship a migration.
- Re-enable `postconfig`, or replace it with something better — the VCF-prereq
  automation is the project's differentiator and it's currently dark.

### Phase 3 — improvements neither repo has

- **One-shot tokens for `ks.cfg`.** Put a nonce in the boot.cfg kernelopt URL,
  bind it to the host + a short TTL, burn it on use. Removes the "root password
  to anyone on the VLAN" exposure.
- **Postgres/MySQL support** alongside SQLite. GORM makes this cheap and it
  unblocks HA. Note `api/pools.go` uses `NOW()` in raw SQL, which is not SQLite
  — that bug is already there and would be fixed by testing on both.
- **Structured OpenAPI.** The swagger docs are generated but stale; regenerate in
  CI and fail on drift. Then a Terraform provider or Ansible module becomes easy —
  and the commit log shows the author already restructured the bootdisk model
  "due to terraform", so there's demand.
- **Modern ESXi 8/9 validation.** The kickstart and boot.cfg logic dates from the
  6.7/7.0 era. Verify against current builds and add version-aware handling.
- **Prometheus metrics + healthz.** Deployments per hour, failures, DHCP packets
  seen, current in-flight installs.
- **IPv6 / DHCPv6**, and BIOS PXE for old hardware — both listed as never-supported
  in the README.
- **Multi-arch container images** (the current Dockerfile is `FROM golang:1.16`
  with a copied binary — should be a multi-stage build on distroless/alpine).

---

## How this analysis was done

```bash
git clone https://github.com/maxiepax/go-via.git
git clone https://github.com/maxiepax/go-via2.git
git -C go-via worktree add ../go-via-dev origin/dev

# build verification
(cd go-via      && go build ./...)   # ok
(cd go-via-dev  && go build ./...)   # ok
(cd go-via2     && go build ./...)   # fails: no go.sum
(cd go-via2     && go mod tidy && go build ./...)   # fails: unused import

# vulnerability scan
govulncheck ./...   # 28 reachable, main and dev alike
```

Repo metadata (stars, forks, license, push dates) via the GitHub API.

`maxiepax/go-via`'s 5 open issues were reviewed separately and contain nothing
that needs addressing, so the roadmap above stands as written.
