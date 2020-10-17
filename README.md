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
    <name of assignemnt>:
      assignmentpath: <gitlab subgroup of semesterpath (or coursepath, if semesterpath is empty)
                       used for assignment>
      # also optional
      description: <project description> # optional
      per: <student|group> # generate per student (default) or per group
      startercode:
        url: <url to repo> # only via SSH atm
      # accesslevel should be guest, developer, reporter, maintainer
      # if not defined accesslevel is developer
      accesslevel: <accesslevel for students>
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
    # accesslevel: developer # default
```

## Usage

```
Manage GitLab for student assignments

Usage:
  glabs [command]

Available Commands:
  check       check course config
  generate    Generate repositories for each student.
  help        Help about any command
  show-config Show config of a course
  version     Print the version number of Glabs

Flags:
      --config string   config file (default is $HOME/.glabs.yml)
  -h, --help            help for glabs
  -v, --verbose         verbose output

Use "glabs [command] --help" for more information about a command.
```

Before generating check wheter all students exist or not using the command

```
glabs check [course]
```
