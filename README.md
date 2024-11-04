# glabs - GitLab-Labs

### Manage GitLab for student labs from the command line

- [glabs - GitLab-Labs](#glabs---gitlab-labs)
    - [Manage GitLab for student labs from the command line](#manage-gitlab-for-student-labs-from-the-command-line)
  - [Configfiles](#configfiles)
    - [Main config file](#main-config-file)
    - [Course config file](#course-config-file)
  - [Usage](#usage)
  - [Generating Reports of Assignments](#generating-reports-of-assignments)
  - [Cloning Repos](#cloning-repos)
  - [Seeding using a custom tool](#seeding-using-a-custom-tool)
    - [Example using `seeder` Option](#example-using-seeder-option)
  - [Using starter code as a template](#using-starter-code-as-a-template)

## Configfiles

### Main config file

Default: `$HOME/.glabs.yml` or whatever format [viper](https://github.com/spf13/viper) can handle.

Contents:

```.yaml
gitlab:
  host: <URL of GitLab host>
  token: <personal access token>
sshprivatekey: <optional path to an unencrypted ssh private key; uses ssh-agent otherwise>
coursesfilepath: <path where config files for courses are>
courses:
  - <basenames of coursesfiles>
```

Example:

```.yaml
gitlab:
  host: https://gitlab.mydomain.nz
  token: abced871263876132jkd
coursesfilepath: $HOME/HM/labs/20WS
courses:
  - algdati
  - vss
```

Placeholders like `$HOME` are not substituted but taken verbatim. It is therefore best to specify absolute paths.

### Course config file

Each course file must be named like the entries in the main config file, e.g., `algdatii.yml` and `vss.yml` in the above example.

Students can be specified by the following:

-   the GitLab user ID (if it is an int)
-   an email address (if it is valid)

**Please do no longer specify students by their GitLab username.**

Contents:

```.yaml
<baseNameOfCourse>:
    # can contain slashes for gitlab subgroups
    coursepath: <base path of course in gitlab>
    # if you want to generate directly in coursepath, do not define semesterpath
    semesterpath: <gitlab subgroup of coursepath used for this semester>
    students: # needs only to defined if generating per student
      <array of student specifier>
    groups: # if students are allowed to work in groups
      <name of fst group>:
        <array of student in group>
      <name of snd group>:
        <array of student in group>
      ...
    <name of assignment>:
      assignmentpath: <gitlab subgroup of semesterpath (or coursepath, if semesterpath is empty)
                       used for assignment>
      # also optional
      description: <project description> # optional
      per: <student|group> # generate per student (default) or per group
      containerRegistry: <false|true> # enable container registry, default false
      startercode:
        url: <url to repo> # only via SSH atm
        fromBranch: <branchName in startercode> # default main
        toBranch: <branchName in generated repo> # default main
        devBranch: <branchName used as default branch> # default toBranch
        protectToBranch: <false|true> # whether only maintainer can push, default false
      # accesslevel should be guest, developer, reporter, maintainer
      # if not defined accesslevel is developer
      accesslevel: <accesslevel for students>
      # It is possible to seed repositories using a custom tool instead of using a startercode.
      seeder:
        cmd: <path to seeding tool> # e.g. python
        args:
          - <list of arguments passed. %s gets replaced by the path of the repository>
        name: <name of the author used for commit>
        email: <email of the author used for commit>
        toBranch: <branch to commit to> # default main
        additionalBranches: # push more than one branch to new repos
          <array of branch names>
        signKey: <plaintext private key for signing commits> # Optional key for signing the commit. If the key is encrypted the password will be requested on running the tool.
        protectToBranch:  <false|true> # whether only maintainer can push, default false
      clone:
        localpath: <local base path for repositories to clone in> # default "."
        branch: <checkout branch> # default main
        force: <false|true> # remove directory if it exists. default false
      release: # to be included in the report
        mergeRequest: # define if it is necessary to release with a merge request
          source: <source branch> # default `develop`
          target: <target branch> # default `main`
          pipeline: <false|true> # default false
        dockerImages: # define if it is necessary to release docker images
          <array of docker image names>
      students: <add students to global student list, which might be empty>
      groups: <add or redefine groups>
```

Example:

```.yaml
algdati:
  coursepath: algdati
  semesterpath: semester/ob-20ws
  students:
    - 12334         # GitLab ID
    - ob@glabs.io   # email address
    - daCoolestName # GitLab username
  groups:
    grp01:
      - hugo@glabs.inc
      - 2345
    grp02:
      - su@u.du
      - allen@u2.du
  blatt0:
    assignmentpath: blatt0
    per: group
    description: Blatt 0, Algorithmen und Datenstrukturen I, WS 20/21
    startercode:
      url: git@gitlab.lrz.de:algdati/startercode/startercodeBlatt1.git
      fromBranch: ws20
      protectToBranch: true
      additionalBranches:
        - ss20
        - 22ss
        - main
    # accesslevel: developer # default
    clone:
      localpath: /tmp
      branch: develop
      clone: true
    grp00:
      - stan
    grp01:
      - hugo
      - sandra
      - quentin
  assignemnt1:
    assignmentpath: a1
    release:
      mergeRequest:
        source: develop
        target: main
        pipeline: true
      dockerImages:
        - customer
        - book
    per: student
```

## Usage

```
Manage GitLab for student assignments

Usage:
  glabs [command]

Available Commands:
  check       check course config
  clone       Clone repositories.
  delete      Delete repositories.
  generate    Generate repositories.
  help        Help about any command
  protect     Protect branch for exisiting repositories.
  report      generate activity report
  setaccess   Set access level for exisiting repositories.
  show        Show config of an assignment
  update      Update repositories with code.
  version     Print the version number of Glabs

Flags:
      --config string   config file (default is $HOME/.glabs.yml)
  -h, --help            help for glabs
  -v, --verbose         verbose output

Use "glabs [command] --help" for more information about a command.
```

Before generating check whether all students exist or not using the command

```
glabs check [course]
```

## Generating Reports of Assignments

It is possible to generate reports in plain text (by default), HTML or JSON via the
`report` command:

```
Generate activity report

Usage:
  glabs report course assignment [flags]

Flags:
  -e, --export-default-template   export the default HTML template
  -h, --help                      help for report
      --html                      generate HTML
      --json                      generate JSON
  -o, --output string             output to <file>
  -t, --tmpl string               use template for HTML
```

If you do not like the default plain text or HTML template feel free to use your
own template via `-t`. You can simply export the default template via `-e` and adapt
it to your needs.

All available report information which can be used in the templates, can be found
[here](https://github.com/obcode/glabs/blob/master/gitlab/report/types.go).

## Cloning Repos

```
Clone repositories for each student or group in course for assignment.
                You can specify students or groups in order to clone only for these.

Usage:
  glabs clone course assignment [groups...|students...] [flags]

Flags:
  -b, --branch string   checkout branch after cloning
  -h, --help            help for clone
  -p, --path string     clone in this directory

Global Flags:
      --config string   config file (default is $HOME/.glabs.yml)
  -v, --verbose         verbose output
```

Command line options (`-b` and `-p`) override the config file settings.

## Seeding using a custom tool

Instead of providing each student/group the same repository using the startercode option it is possible to run a tool to seed each repository individually.

1. Therefore a new repository gets created at the `clone.localpath` location.
2. Afterwards the tool runs inside this location. The argument `%s` gets replaced by the location e.g. helpfull storing some solutions for the matching repo somewhere else.
3. The files get added to the repository.
4. The changes get commited using `seeder.name` and `seeder.email`.
5. Finally the changes get pushed to the remote location.

### Example using `seeder` Option

```.yaml
algdati:
  coursepath: algdati
  semesterpath: semester/ob-20ws
  students:
    - olli
    - ob
    - obcode
  blatt0:
    assignmentpath: blatt0
    per: group
    description: Blatt 0, Algorithmen und Datenstrukturen I, WS 20/21
    seeder:
      cmd: python
      args:
        - /data/repos/generate.py
        - generate-keys
        - %s
      name: Your Name
      email: foo@bar.com
      toBranch: main
    clone:
      localpath: /tmp
```

## Using starter code as a template

Currently glabs does not support rewriting git history.

What you can do is the following:

1. Clone your starter code repository.
2. Create a new branch. Example:

    ```
    $ git checkout -B ws24
    ```

3. Commit the whole tree with `commit-tree`. Be sure to remember the the
   commit object id. Example:

    ```
    $ git commit-tree HEAD^{tree} -m "Initial"
    6439f935612064028d6678c457991660cfe7e15e
    ```

4. Reset the current branch to the new commit. Example:

    ```
    git reset 6439f935612064028d6678c457991660cfe7e15e
    ```

5. Push the new branch to origin. Example:

    ```
    git push origin ws20
    ```

6. Use `fromBranch` in your assignment config.
