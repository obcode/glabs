# Command Reference

Use this page as quick reference. For details, run help on each command.

## Global usage

```text
glabs [command] [flags]
```

### Global flags

- `--config string`: path to main config file
- `-v, --verbose`: enable verbose logging
- `-h, --help`: show help

## Main commands

### generate

Generate repositories for students or groups in an assignment.

```text
glabs generate <course> <assignment> [groups...|students...]
```

Creates all project repositories based on config. If startercode is configured, it clones from the source repository.

### check

Validate course and assignment configuration.

```text
glabs check <course>
```

Validates all assignments and lists details. Run before `generate` to catch config errors.

### protect

Apply branch protection rules to existing repositories.

```text
glabs protect <course> <assignment> [groups...|students...]
```

Useful after updating protection settings in config. Does not create repositories.

### clone

Clone repositories locally.

```text
glabs clone <course> <assignment> [groups...|students...] [flags]
```

**Flags:**

- `-b, --branch string`: checkout specific branch (default: main)
- `-p, --path string`: target directory (default: .)
- `-f, --force`: remove existing directory before cloning
- `-s, --suppress`: print only local path (useful with scripts)

### urls

Print repository URLs.

```text
glabs urls <course> <assignment> [groups...|students...]
```

Useful with `| xargs open` to open all repos in browser.

### delete

Delete repositories permanently.

```text
glabs delete <course> <assignment> [groups...|students...]
```

⚠️ **Warning**: Asks for confirmation before deleting. Repositories are deleted permanently and cannot be recovered.

### update

Update repositories with new code from startercode source.

```text
glabs update <course> <assignment> [groups...|students...]
```

⚠️ **Warning**: USE WITH CARE! Only works with unmodified repositories. Merge conflicts cannot be resolved automatically. Only use if you know students haven't started working yet.

### report

Generate activity or submission reports.

```text
glabs report <course> <assignment> [flags]
```

**Flags:**

- `--html`: generate HTML report
- `--json`: generate JSON report
- `-t, --template string`: custom template file

**Default**: plain text report

Export template:

```sh
glabs report <course> <assignment> -e
```

### setaccess

Set or change access level for existing repositories.

```text
glabs setaccess <course> <assignment> [groups...|students...] [flags]
```

**Flags:**

- `-l, --level string`: override access level (guest, reporter, developer, maintainer)

Useful for adjusting permissions after repositories are created.

### archive

Archive or unarchive repositories.

```text
glabs archive <course> <assignment> [groups...|students...] [flags]
```

**Flags:**

- `-u, --unarchive`: unarchive instead of archive

Archived projects are hidden in GitLab UI but still accessible.

### show

Display resolved assignment configuration.

```text
glabs show <course> <assignment> [groups...|students...]
```

Shows how glabs interprets your config after merging course-level and assignment-level settings. Useful for debugging.

### version

Print glabs version.

```text
glabs version
```

## push

You can push one deferred branch at a time to all student/group repositories using the `push` command. You can define more than one deferred branch.

### Assignment Config Example

```yaml
deferredBranches:
  solution:
    url:           # (optional) source repo, defaults to startercode URL
    fromBranch:    # (default: solution)
    toBranch:      # (default: solution)
    orphan:        # (default: true)
    orphanMessage: # (default: Snapshot of solution)
  anotherbranch:
    url:           # (optional) source repo, defaults to startercode URL
    fromBranch:    # (default: anotherbranch)
    toBranch:      # (default: anotherbranch)
    orphan:        # (default: true)
    orphanMessage: # (default: Snapshot of anotherbranch)
```

- If `orphan: true`, a new orphan branch is created in each repo with a single commit from the deferred branch.
- If `orphan: false`, the deferred branch is pushed as a normal branch (with complete history).
- deferred branches are always pushed with `--force`

**Usage:**

```sh
glabs push <course> <assignment> <deferred-branch> [groups...|students...]
```

e.g.

```sh
glabs push mpd ass1 solution
```

If you are using `orphan` you might want to set the committer name and email in `~/.glabs.yaml` like so

```
committer:
  name: Example User
  email: user@example.com
```

## Filtering students or groups

When specifying `[groups...|students...]`, patterns are treated as regular expressions:

```sh
# Exact match
glabs generate mpd blatt01 alice

# Pattern match (all students with 'a' in name)
glabs generate mpd blatt01 'a.*'

# Multiple patterns
glabs generate mpd blatt01 'a.*' 'b.*'
```

## Common syntax patterns

**By course and assignment only:**

```text
glabs generate <course> <assignment>
```

**Filter by name/pattern:**

```text
glabs generate <course> <assignment> <name-or-regex>
```

**Multiple filters:**

```text
glabs generate <course> <assignment> <pattern1> <pattern2> ...
```

**With flags:**

```text
glabs clone <course> <assignment> -p /tmp/work -b develop
glabs report <course> <assignment> --html -t template.tmpl
```

## Get help

```sh
glabs --help                  # Show all commands
glabs generate --help         # Show generate help with flags
glabs protect --help          # Show protect help
glabs report --help           # Show report help
glabs -v generate mpd blatt01 # Run with verbose logging
