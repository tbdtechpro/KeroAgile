# Notes, links and kept files

A ticket collects three kinds of record as it is worked:

- **Notes**: dated, signed entries. They are only ever added, so several people or agents can write without overwriting each other.
- **Links** to the commits, branches, pull requests and tags that implement it. Most are recorded automatically from commit messages that name the ticket.
- **Kept files**: test scripts, logs, screenshots, audio and video produced while working it, which would otherwise be lost when a session's scratch folder is cleared.

Records are kept in the local database. The cross-machine sync does not carry them.

## Notes

```bash
KeroAgile task note KA-012 "Reproduced on a 24-row terminal; the header scrolls off."
KeroAgile task get KA-012          # prints the notes
```

In the TUI, press `a` on a ticket. In the web UI, use the box under the notes. Through MCP, use `add_note`.

## Links

```bash
KeroAgile task link KA-012 --commit 34cbece
KeroAgile task link KA-012 --pr 41
KeroAgile git scan                 # link this repository's commits and PRs
```

`git scan` reads the checked-out branch and links each commit whose message names a ticket of this repository's project (for example `fix: header overflow (KA-012)`). Every ten minutes it also asks `gh` for pull requests whose title or branch names a ticket, and records their state. It never links another project's tickets and never removes a link. A commit that was rebased or squashed away is marked `superseded`.

For a scan to work, the project needs its checkout directory:

```bash
KeroAgile project set KA --dir ~/code/myapp
KeroAgile project which            # run inside the checkout: which project is this?
```

## Kept files

```bash
KeroAgile artifacts attach KA-012 ./run.log --note "the passing run"
KeroAgile artifacts list --task KA-012
KeroAgile artifacts fetch 7 --to ./copy.log
KeroAgile artifacts status
```

Files are copied into a holding area beside the database (`~/.config/keroagile/artifacts` by default). Each distinct content is stored once, named by its SHA-256. Through MCP, Claude uses `attach_file`. The MCP server also tells Claude to attach what it produces before finishing a ticket.

### The sweep: a safety net

Claude may not attach everything. A Claude Code Stop hook runs after every turn. It keeps the new or changed files in the session's scratch folder as **unfiled** files of the project the session is working in. It also runs `git scan`. Add it once, at user level, so it covers every project:

```bash
KeroAgile hook print               # prints the snippet for ~/.claude/settings.json
```

The hook needs Claude Code 2.1.257 or later, and it always exits successfully. It does nothing for sessions outside any project's directory; `artifacts status` lists those directories. The sweep skips:

- dependency and cache folders;
- files whose names look like secrets (`.env`, keys, credentials, `*token*`);
- symlinks;
- files over `sweep_max_file`;
- files still being written.

A file that keeps changing (a growing log) stays one unfiled file holding its latest contents. File or discard unfiled files:

```bash
KeroAgile artifacts list --unfiled
KeroAgile artifacts file 9 KA-012
KeroAgile artifacts discard 10     # the sweep will not keep it again
```

### Room

| Setting | Default | Meaning |
|---|---|---|
| `local_max` | `1GiB` | Most the holding area may hold |
| `min_free` | `2GiB` | Never take the disk below this |
| `keep_local_under` | `256KiB` | Files this small stay here even once uploaded |
| `max_file` | `2GiB` | Larger files are refused |
| `sweep_max_file` | `100MiB` | The sweep skips larger files; attach them deliberately |
| `keep_unreferenced` | `14d` | How long contents nothing uses are kept |

When a file would not fit, KeroAgile first removes what it safely can:

1. contents nothing uses any more;
2. uploaded files larger than `keep_local_under`.

If that is not enough, the new file is refused with a message. Nothing already kept is dropped.

### Uploading large files elsewhere

Each project has a policy:

- `remote`: keep, and upload.
- `local`: keep on this machine only.
- `off`: keep nothing.
- unset: follow `default_policy`.

`default_policy` is `local` unless you change it, so configuring a remote does not by itself send anything anywhere:

```bash
KeroAgile project set KA --artifact-policy remote
```

`KeroAgile serve` uploads every minute. Run `KeroAgile artifacts sync` to upload now. A file is marked uploaded only after the store confirms it holds it. Nothing is deleted from the remote unless you run `KeroAgile artifacts prune --apply`. Without `--apply`, `prune` only lists what would go.

**Which remote to pick**

| You have | Use | Notes |
|---|---|---|
| A server, a NAS with an S3 app, MinIO, Garage, or a cloud bucket | `s3` | Works request by request, so it tolerates a poor or intermittent link. Best when you work away from home over a VPN |
| A NAS share, a second disk, or a synced folder (Dropbox, OneDrive, iCloud Drive) already mounted | `folder` | Simplest. Avoid a network share over a link that drops: a stalled mount blocks whatever touches it. KeroAgile gives up after a timeout and retries later |

**S3-compatible store**

```toml
[artifacts]
remote = "s3"

[artifacts.s3]
endpoint   = "http://192.0.2.10:3900"   # your store
region     = "garage"                   # whatever your store expects; AWS default us-east-1
bucket     = "keroagile"
path_style = true                       # true for most self-hosted stores
profile    = "keroagile"                # a profile in ~/.aws/credentials
```

Credentials never go in KeroAgile's config. Put them in the shared AWS credentials file (`~/.aws/credentials`, or the path in `AWS_SHARED_CREDENTIALS_FILE`):

```ini
[keroagile]
aws_access_key_id     = GK...
aws_secret_access_key = ...
```

Or set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` and leave `profile` out. Give KeroAgile its own bucket, and a key that can reach only that bucket. Set a size limit (quota) on the bucket if your store supports one. Plain `http://` sends file contents unencrypted; use it only on a network you trust.

**Folder**

```toml
[artifacts]
remote = "folder"

[artifacts.folder]
path = "/mnt/nas/keroagile"        # Linux
# path = "/Volumes/nas/keroagile"  # macOS
# path = 'Z:\keroagile'            # Windows
```

Objects are stored under an ID unique to each KeroAgile database, so two machines can share one bucket or folder without touching each other's files.
