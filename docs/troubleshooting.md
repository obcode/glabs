# Troubleshooting

## Configuration and startup

### Config file not found

**Symptoms:**
```
panic: fatal error config file: no such file or directory
```

**Checks:**
- Config file exists at `~/.glabs.yaml`
- If using custom path: `--config /path/to/config.yaml`
- File is valid YAML (check indentation)
- `coursesfilepath` directory exists
- All course files listed in `courses` exist in `coursesfilepath`

**Test:**
```sh
glabs check <course>  # Validates config
```

### Course not found

**Symptoms:**
```
fatal error config file: configuration for course not found
```

**Checks:**
- Course listed in main config `courses` section
- Course filename matches (e.g., `mpd.yaml` for `courses: [mpd]`)
- Course config file is in `coursesfilepath`
- Course has top-level key matching the filename

**Example:**
```yaml
# main config lists: courses: [mpd]
# file should be: ~/courses/mpd.yaml
# with content: mpd:  <-- key must match filename
```

### Assignment not found

**Symptoms:**
```
configuration for assignment not found
```

**Checks:**
- Assignment exists in course file
- Assignment key matches what you typed
- Indentation is correct (YAML)

## GitLab authentication

### Token or auth issues

**Symptoms:**
```
401 Unauthorized
invalid API token
```

**Checks:**
- Token exists in main config under `gitlab.token`
- Token has **API scope** (check GitLab UI)
- Token is not expired
- `gitlab.host` URL is correct
- Can curl the API: `curl -H "Authorization: Bearer TOKEN" https://gitlab.lrz.de/api/v4/user`

**Test:**
```sh
glabs check <course>  # Validates token access
```

### SSH key issues (startercode)

**Symptoms:**
```
Permission denied (publickey)
ssh: connect to host gitlab.lrz.de port 22: Connection refused
```

**Checks:**
- SSH key loaded: `ssh-add -l`
- Key has permission to read starter repo
- URL is SSH format: `git@gitlab.lrz.de:...` (not https)
- Can clone manually: `git clone git@gitlab.lrz.de:...`

**Configure custom key:**
```yaml
# main config
sshprivatekey: /path/to/id_rsa
```

## Repository generation

### Generate fails silently

**Symptoms:**
```
glabs generate mpd blatt01
# Returns without creating repos
```

**Checks:**
- Run with verbose logging: `glabs -v generate mpd blatt01`
- Check `coursepath` and `assignmentpath` are correct
- GitLab groups exist: navigate to `gitlab.host/course/semester/assignment` path
- Student/group list is not empty: `glabs show mpd blatt01`

**Tip:** Create groups manually in GitLab if they don't auto-create

### Generate fails with merge conflicts

**Symptoms:**
```
merge conflict
cannot merge branches
```

**Checks:**
- `fromBranch` exists in starter repository
- `toBranch` branch doesn't already exist in student repo (first run?)
- No conflicting files if overwriting

**Workaround:**
```sh
# Delete repos and regenerate
glabs delete mpd blatt01
glabs generate mpd blatt01
```

## Branch protection

### Branch protection not as expected

**Symptoms:**

In GitLab UI you see wrong permissions or "Allowed to push and merge" instead of merge-only.

**Verification:**

```sh
# Reapply protection rules
glabs protect <course> <assignment>
```

Then check GitLab UI for expected behavior.

**For merge-only dev branch:**

Expected:
- **Allowed to merge**: Developers and Maintainers
- **Allowed to push and merge**: No one

If wrong, check config:

```yaml
branches:
  - name: main
    mergeOnly: true  # Must be true
    default: true
```

Note: `mergeOnly` applies exactly to the branch specified in `name`.

## Modify existing repositories

### Update fails with merge conflicts

**Symptoms:**
```
cannot resolve merge conflict
```

⚠️ **Important**: `glabs update` only works with **unmodified** repositories.

**Use only when:**
- Early semester, before students work
- Small fixes to template
- You know repositories are untouched

**Avoid when:**
- Students have already worked
- Significant code changes
- Unsure about conflicts

**Alternative:**
Delete and regenerate:
```sh
glabs delete mpd blatt01
glabs generate mpd blatt01
```

### Setaccess fails

**Symptoms:**
```
error changing access level
```

**Checks:**
- Access level is valid: `guest`, `reporter`, `developer`, `maintainer`
- You have maintainer permission on projects
- Repositories exist (create first with `generate`)

**Test:**
```sh
glabs show mpd blatt01  # Verify config
glabs setaccess mpd blatt01 -l reporter  # Change level
```

### Delete prompts but doesn't delete

**Symptoms:**
```
Do you really want to delete? Press Enter...
# Pressing Enter doesn't delete
```

**Checks:**
- Confirm deletion by pressing Enter (not other keys)
- You have maintainer permission on projects
- Terminal is not blocked (check for background processes)

**Alternative:** Use `^C` to cancel and try again

### Archive fails or unarchive fails

**Symptoms:**
```
error archiving project
```

**Checks:**
- Repositories exist
- You have maintainer permission
- Project is not already in desired state

**Verify:**
```sh
glabs show mpd blatt01
```

## Clone operations

### Clone fails because path exists

**Symptoms:**
```
error: destination path already exists
```

**Solution:**
```sh
glabs clone mpd blatt01 -f  # -f = force, removes existing
```

Or specify different path:
```sh
glabs clone mpd blatt01 -p /tmp/repos
```

### Clone wrong branch

**Symptoms:**
```
glabs clone mpd blatt01
# Clones main instead of develop
```

**Solution:**
```sh
glabs clone mpd blatt01 -b develop
```

Or set in config:
```yaml
clone:
  branch: develop
```

## Reports

### Report template issues

**Symptoms:**
```
error parsing template
undefined variable
```

**Steps:**

1. Export default template:
```sh
glabs report mpd blatt01 -e > template.tmpl
```

2. Inspect available fields
3. Modify your template
4. Use custom template:
```sh
glabs report mpd blatt01 --html -t template.tmpl
```

### Report missing data

**Symptoms:**
```
report shows empty fields
no commits visible
```

**Checks:**
- Repositories exist and have activity
- GitLab API token has read access
- `glabs report mpd blatt01` works without flags first
- Check verbose mode: `glabs -v report mpd blatt01`

## Seeder issues

### Seeder script not found

**Symptoms:**
```
command not found
cannot open script
```

**Checks:**
- Script path is absolute (required)
- Script is executable: `chmod +x script.py`
- Command exists: `which python` or `which python3`

**Example config:**
```yaml
seeder:
  cmd: python3
  args:
    - /absolute/path/to/script.py
    - "%s"
```

### Seeder GPG passphrase issues

**Symptoms:**
```
[prompts for passphrase but gets stuck]
[error: decryption failed]
```

**Checks:**
- GPG key is valid and properly formatted (-----BEGIN...)
- Passphrase is correct
- GPG is installed: `gpg --version`

**Debug:**
```sh
glabs -v generate mpd blatt01  # More details
```

### Seeder commits fail

**Symptoms:**
```
error committing changes
author name invalid
```

**Checks:**
- `name` and `email` are set in seeder config
- Email format is valid
- Git can commit locally

**Test:**
```sh
git config user.name "Bot Name"
git config user.email "bot@example.org"
```

## Network and permissions

### API rate limit exceeded

**Symptoms:**
```
429 Too Many Requests
rate limit exceeded
```

**Solution:**
- Wait (limits reset per hour)
- Use different GitLab token
- Check if another process is hammering the API

**Reduce requests:**
```sh
glabs check <course>  # Validates without API calls where possible
```

### Permission denied on projects

**Symptoms:**
```
403 Forbidden
you do not have permission
```

**Checks:**
- GitLab token user has maintainer access to projects
- Token was created by appropriate user
- Coursepath groups exist and are owned correctly

## Debug mode

Enable verbose logging for all commands:

```sh
glabs -v <command> <args>
```

**Example:**
```sh
glabs -v generate mpd blatt01 alice
glabs -v protect mpd blatt01
glabs -v report mpd blatt01 --html
```

Verbose output shows:
- Config loading steps
- API calls and responses
- Student/group matching
- Branch operations
- Detailed error messages

## Still stuck?

1. **Run `glabs show`** to see resolved config
2. **Run with `-v` flag** to see debug details
3. **Check GitLab UI** manually for what exists
4. **Verify paths** in config are absolute paths
5. **Confirm GitLab groups** exist and are accessible
6. **Read full command help**: `glabs <command> --help`
