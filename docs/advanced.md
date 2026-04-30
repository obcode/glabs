# Advanced Topics

This guide covers more complex workflows and best practices.

## Release workflow with GitLab Flow

GitLab Flow uses two main branches:
- **develop**: Feature development (students work here)
- **main**: Production code (released version)

### Setup release workflow

```yaml
mpd:
  blatt02:
    startercode:
      url: git@gitlab.lrz.de:mpd/starter.git
      fromBranch: startercode
      toBranch: main
      template: true
      templateMessage: Initial Startercode

    branches:
      - name: main
        protect: true              # main is locked
      - name: develop
        default: true

    release:
      mergeRequest:
        source: develop            # PR from this branch
        target: main               # Into this branch
        pipeline: true             # Require passing CI
      dockerImages:
        - myapp/service
```

### Process

1. Students work on `develop` branch
2. When ready for submission: create merge request `develop` → `main`
3. If `pipeline: true`, CI/CD must pass first
4. Merge to `main`
5. Docker images automatically built if configured

### View merge requests

```sh
glabs urls mpd blatt02 | xargs open  # Open all repos
# Then navigate to Merge Requests tab in each
```

## Container registry and Docker

Enable container registry for automated builds:

```yaml
release:
  dockerImages:
    - myapp/backend
    - myapp/frontend
```

### What glabs does

- Creates container registry entries
- Enables Docker build configurations
- Sets up image namespaces

### Student workflow

1. Push Dockerfile(s) to repository
2. Commit to `main` branch
3. GitLab CI/CD builds images automatically
4. Images available in project container registry

### Example Dockerfile

```dockerfile
FROM python:3.11-slim

WORKDIR /app
COPY . /app

RUN pip install -r requirements.txt

CMD ["python", "app.py"]
```

## Custom seeding with scripts

Use seeder when startercode alone isn't enough.

### Simple Python generator

**config/mpd.yaml:**
```yaml
seeder:
  cmd: python3
  args:
    - /path/to/generate_assignment.py
    - "%s"
  name: Course Bot
  email: bot@hm.edu
  toBranch: main
```

**generate_assignment.py:**
```python
#!/usr/bin/env python3
import os
import sys
import subprocess

repo_path = sys.argv[1]

# Create directory structure
os.makedirs(f"{repo_path}/src", exist_ok=True)
os.makedirs(f"{repo_path}/tests", exist_ok=True)

# Generate files
with open(f"{repo_path}/README.md", "w") as f:
    f.write("# Assignment\n\n")

with open(f"{repo_path}/src/main.py", "w") as f:
    f.write("# TODO: Implement your solution here\n")

# Commit changes
os.chdir(repo_path)
subprocess.run(["git", "add", "."], check=True)
subprocess.run([
    "git", "commit", "-m", "Initial commit from seeder",
    "--author", "Course Bot <bot@hm.edu>"
], check=True)
```

### Seeder with GPG signing

For verified commits, use GPG signing:

```yaml
seeder:
  cmd: python3
  args:
    - /path/to/seeder.py
    - "%s"
  name: Course Bot
  email: bot@hm.edu
  signKey: |
    -----BEGIN PGP PRIVATE KEY BLOCK-----
    [base64-encoded GPG key]
    -----END PGP PRIVATE KEY BLOCK-----
```

**When running `generate`**, you'll be prompted:
```
Passphrase for signing key is required. Please enter it now:
```

**Create GPG key for signing:**

```sh
# Generate key
gpg --full-generate-key

# Export key (needed for config)
gpg --armor --export-secret-key bot@hm.edu > private-key.asc

# View content for config file
cat private-key.asc
```

**Students will see:**
```
✓ Signed commit (verified badge in GitLab UI)
```

## Multi-course setup

Manage multiple courses with shared configuration.

### Directory structure

```
~/.glabs.yaml          # Main config

~/courses/
  mpd.yaml             # Course 1
  vss.yaml             # Course 2
  shared.yaml          # Shared definitions (not in courses list)
```

### Main config

```yaml
gitlab:
  host: https://gitlab.lrz.de
  token: glpat-XXXXX

coursesfilepath: ~/courses
courses:
  - mpd
  - vss
```

### Share student lists

**shared.yaml (NOT in courses list):**
```yaml
shared_students:
  - alice@hm.edu
  - bob@hm.edu
```

**mpd.yaml:**
```yaml
mpd:
  coursepath: teaching/mpd
  students: ${shared_students}  # Reference doesn't work
```

**Workaround:** Define in both course files or use course-level override per assignment.

## Batch operations with scripting

Use glabs with shell scripts for complex workflows.

### Generate all assignments in course

```bash
#!/bin/bash
course=$1
shift
assignments=$@

for assignment in $assignments; do
  echo "Generating $course $assignment..."
  glabs generate $course $assignment || echo "Failed: $assignment"
done
```

**Run:**
```sh
./batch_generate.sh mpd blatt01 blatt02 blatt03
```

### Clone and report

```bash
#!/bin/bash
course=$1
assignment=$2

# Clone all
glabs clone $course $assignment -f -p /tmp/work

# Run analysis
for repo in /tmp/work/*; do
  student=$(basename "$repo")
  commits=$(cd "$repo" && git rev-list --count HEAD)
  lines=$(cd "$repo" && find . -name "*.py" | xargs wc -l | tail -1)
  echo "$student: $commits commits, $lines lines"
done

# Generate report
glabs report $course $assignment --html > report.html
```

## Performance tips

### For large courses (100+ students)

**1. Batch generate by group**
```sh
glabs generate mpd blatt01 'team-a'  # One group
glabs generate mpd blatt01 'team-b'  # Another group
```

**2. Skip verbose logging**
```sh
glabs generate mpd blatt01  # Default (no -v flag)
```

**3. Filter before operations**
```sh
glabs clone mpd blatt01 'a.*'  # Only names starting with 'a'
```

### For expensive operations

Run during off-peak hours:
```sh
# Late night: generate all
glabs generate mpd blatt01

# Next morning: protect and report
glabs protect mpd blatt01
glabs report mpd blatt01 --html
```

## Automation with cron

Run glabs operations on schedule.

### Daily report generation

**crontab -e:**
```cron
# Generate reports daily at 6 AM
0 6 * * * cd /home/instructor && glabs report mpd blatt01 --html > reports/blatt01-$(date +\%Y\%m\%d).html
```

### Semester workflow

**crontab -e:**
```cron
# Generate repos on day 1
0 8 1 * * glabs generate mpd blatt01
0 9 1 * * glabs generate mpd blatt02

# Protect branches on submission day
0 18 3 * * glabs protect mpd blatt01
0 18 10 * * glabs protect mpd blatt02

# Generate reports daily during submission period
0 23 * * * glabs report mpd blatt01 --html
```

## Integration with CI/CD

Use glabs output in GitLab CI/CD pipelines.

### Get URLs for deployment

```yaml
# .gitlab-ci.yml
deploy_stage:
  script:
    # Get list of student repos
    - glabs urls mpd blatt01 > repo_urls.txt

    # Deploy to each
    - |
      while read url; do
        git clone $url && cd submission && ./deploy.sh
      done < repo_urls.txt
```

### Automated testing

```yaml
test_submissions:
  script:
    # Clone all submissions
    - glabs clone mpd blatt01 -f -p /tmp/submissions

    # Run tests
    - |
      for repo in /tmp/submissions/*; do
        cd $repo && ./run_tests.sh || echo "Failed: $repo"
      done
```

## Best practices

### 1. Always validate first

```sh
glabs check <course>      # Before any operation
glabs show <course> <assignment>  # Before risky operations
```

### 2. Test with sample data

Create test students/groups first:
```yaml
students:
  - test1@hm.edu
  - test2@hm.edu
  - alice@hm.edu  # Real students
```

Then test with one first:
```sh
glabs generate mpd blatt01 test1
```

### 3. Backup before batch operations

```sh
# Save current state
glabs report mpd blatt01 --json > backup.json

# Then run operation
glabs update mpd blatt01
```

### 4. Use descriptive repo paths

```yaml
coursepath: teaching/2024ss/mpd  # Include year/semester
assignmentpath: blatt-01-datastructures  # Describe content
```

### 5. Protect production branch always

```yaml
startercode:
  toBranch: main
  protectToBranch: true  # Always lock production
```

### 6. Document in course file

```yaml
# mpd.yaml
# Course: Algorithms and Data Structures
# Semester: 2024 Summer Semester
# Instructor: Prof. Mueller
# GitLab Group: teaching/2024ss/mpd

mpd:
  coursepath: teaching/2024ss/mpd
  # ... rest of config
```

### 7. Version control course configs

```sh
git init ~/courses
git add mpd.yaml vss.yaml
git commit -m "Course config 2024ss"
git remote add origin git@gitlab.lrz.de:teaching/configs.git
git push
```

## Troubleshooting advanced scenarios

### Merge conflicts during seeding

If seeder script fails with merge conflicts:

1. Check if students have existing work
2. Use `glabs delete` and regenerate if needed
3. Or resolve conflicts manually in each repo

### Performance degradation

If glabs becomes slow:

1. Check network (GitLab API latency)
2. Use verbose mode to identify bottlenecks: `glabs -v generate ...`
3. Filter operations: `glabs protect mpd blatt01 'alice'` (one student)
4. Consider chunking large operations

### GitLab API rate limiting

With 100+ students, you may hit rate limits:

1. Spread operations over time
2. Use different API token
3. Contact GitLab admin for higher limits
4. Batch operations: generate morning, protect afternoon

## Getting more help

- **Full help**: `glabs --help` and `glabs <command> --help`
- **See actual config**: `glabs show <course> <assignment>`
- **Debug mode**: `glabs -v <command> ...`
- **Report issues**: Check the [GitHub repository](https://github.com/obcode/glabs)
