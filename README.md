# glabs - GitLab-Labs

### Manage GitLab for student labs from the command line

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

### Course config file

Contents:

```.yaml
<baseNameOfCourse>:
    # can contain slashes for gitlab subgroups
    coursepath: <base path of course in gitlab>
    # if you want to generate directly in coursepath, do not define semesterpath
    semesterpath: <gitlab subgroup of coursepath used for this semester>
    students: # needs only to defined if generating per student
      <array of students account names>
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
        protectToBranch: <false|true> # whether only maintainer can push, default false
      # accesslevel should be guest, developer, reporter, maintainer
      # if not defined accesslevel is developer
      accesslevel: <accesslevel for students>
      clone:
        localpath: <local base path for repositories to clone in> # default "."
        branch: <checkout branch> # default main
        force: <false|true> # remove directory if it exists. default false
      students: <add students to global student list, which might be empty>
      groups: <add or redefine groups>
```

Example:

```.yaml
algdati:
  coursepath: algdati
  semesterpath: semester/ob-20ws
  students:
    - olli
    - ob
    - obcode
  groups:
    grp01:
      - hugo
      - sandra
    grp02:
      - su
      - allen
  blatt0:
    assignmentpath: blatt0
    per: group
    description: Blatt 0, Algorithmen und Datenstrukturen I, WS 20/21
    startercode:
      url: git@gitlab.lrz.de:algdati/startercode/startercodeBlatt1.git
      fromBranch: ws20
      protectToBranch: true
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
```

## Usage

```
Manage GitLab for student assignments

Usage:
  glabs [command]

Available Commands:
  check       check course config
  clone       Clone repositories.
  generate    Generate repositories.
  help        Help about any command
  show        Show config of an assignment
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

## Using starter code as a template

Currently glabs does not support rewriting git history.

What you can do is the following:

1. Clone your starter code repository.
2. Create a new branch. Example:

    ```
    $ git checkout -B ws20
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
