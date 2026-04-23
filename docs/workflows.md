# Workflows

This page contains practical recipes for common tasks.

## Generate repositories

For all students/groups in assignment:

```sh
glabs generate <course> <assignment>
```

For selected students/groups using regex patterns:

```sh
glabs generate <course> <assignment> alice bob 'a.*'
```

## Protect branches for existing repositories

```sh
glabs protect <course> <assignment>
```

Use this after changing branch protection settings in config. Does not regenerate repositories, only updates protection rules.

## Merge-only development branch

Use this startercode config:

```yaml
startercode:
  devBranch: main
  protectDevBranchMergeOnly: true
```

Expected behavior in GitLab UI:

- **Allowed to merge**: Developers and Maintainers
- **Allowed to push and merge**: No one

This allows developers to review and merge code, but cannot push directly.

## Clone repositories locally

```sh
glabs clone <course> <assignment>
```

**Useful flags:**

- `-b, --branch`: checkout specific branch
- `-p, --path`: target directory
- `-f, --force`: remove existing directory first
- `-s, --suppress`: print only local paths (for scripting)

**Examples:**

Clone to specific directory:

```sh
glabs clone mpd blatt01 -p ~/work/submissions
```

Clone and force-remove existing:

```sh
glabs clone mpd blatt01 -f -p /tmp/work
```

Clone only students matching pattern:

```sh
glabs clone mpd blatt01 'a.*' -p /tmp/work
```

## Get repository URLs

```sh
glabs urls <course> <assignment>
```

**Open pages directly in browser:**

```sh
glabs urls <course> <assignment> | xargs open
```

**Save to file:**

```sh
glabs urls <course> <assignment> > urls.txt
```

## Change project access level

Change the access level for existing repositories:

```sh
glabs setaccess <course> <assignment> -l developer
```

**Supported levels:**

- `guest` (10): can view projects
- `reporter` (20): can create issues, pull requests
- `developer` (30): can push code, merge
- `maintainer` (40): full admin access

**Override per assignment:**

```sh
glabs setaccess mpd blatt01 -l reporter
```

Only affect specific students:

```sh
glabs setaccess mpd blatt01 alice bob -l maintainer
```

## Delete repositories permanently

```sh
glabs delete <course> <assignment>
```

⚠️ **Warning**: This asks for confirmation and permanently deletes all repositories. Cannot be undone.

Delete only specific student repos:

```sh
glabs delete mpd blatt01 alice
```

## Update repositories with new code

```sh
glabs update <course> <assignment>
```

⚠️ **USE WITH CARE!** Only works with unmodified repositories. Merge conflicts cannot be resolved automatically.

**When to use:**

- Early in the semester, before students start working
- To push bug fixes to starter code
- Small template changes across all repos

**When NOT to use:**

- After students have worked on code
- If you're unsure about merge conflicts
- In production/live semester

## Archive or unarchive repositories

**Archive (hide from UI):**

```sh
glabs archive <course> <assignment>
```

**Unarchive:**

```sh
glabs archive <course> <assignment> -u
```

Archived projects are hidden in GitLab UI but:
- Data is still there
- Can be unarchived anytime
- Useful for cleanup at semester end

## Generate reports

**Default (plain text):**

```sh
glabs report <course> <assignment>
```

**HTML report:**

```sh
glabs report <course> <assignment> --html
```

**JSON report (for scripts):**

```sh
glabs report <course> <assignment> --json
```

**Custom template:**

1. Export default template:

```sh
glabs report <course> <assignment> -e > template.tmpl
```

2. Modify the template

3. Use custom template:

```sh
glabs report <course> <assignment> --html -t template.tmpl
```

## Release workflow with merge requests and Docker

Use this config for release setup:

```yaml
release:
  mergeRequest:
    source: develop      # Branch with new code
    target: main         # Production branch
    pipeline: true       # Wait for CI/CD to pass
  dockerImages:
    - myapp/backend
    - myapp/frontend
```

When configured, glabs can manage:

- Automatic merge request creation
- Docker image builds
- Container registry integration

This enables a GitLab Flow where development happens on `develop` and production releases are merged to `main`.

## Replicate issues from starter repository

When creating new repositories from startercode, automatically copy issues:

```yaml
startercode:
  replicateIssue: true
  issueNumbers: [1, 3, 7]
```

**Behavior:**

- Only on newly generated repositories
- Copies title and description from starter repo
- Creates new issues in student repos
- Useful for: assignment requirements, bonus tasks, grading checklist

**Examples:**

```yaml
startercode:
  url: git@gitlab.example.org:course/starter.git
  replicateIssue: true
  issueNumbers: [1]  # Replicate first issue (usually assignment spec)
```

## Seed repositories with custom code

For complex seeding (beyond simple startercode copy), use seeder:

```yaml
seeder:
  cmd: python
  args:
    - /path/to/generator.py
    - "%s"              # %s is replaced with repo path
  name: Generator Bot
  email: generator@example.org
  toBranch: main
  protectToBranch: false
```

**The seeder script receives the local path and can:**

- Generate files programmatically
- Create specific folder structures
- Run build commands
- Commit with custom author/email

**With GPG signing:**

```yaml
seeder:
  cmd: python
  args: [/path/to/gen.py, "%s"]
  name: Bot Name
  email: bot@example.org
  signKey: |
    -----BEGIN PGP PRIVATE KEY BLOCK-----
    [base64-encoded key]
    -----END PGP PRIVATE KEY BLOCK-----
```

When you run `generate`, you will be prompted for the GPG passphrase if needed.

## Show resolved configuration

Debug what glabs actually uses:

```sh
glabs show <course> <assignment>
```

This shows the merged configuration after:
- Loading main config (~/.glabs.yaml)
- Loading course config
- Loading assignment config
- Applying defaults

Useful for understanding which values are being used.
