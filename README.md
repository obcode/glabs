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
groupsfilepath: <path where config files for groups are>
groups:
  - <basenames of groupfiles>
```

Example:

```.yaml
gitlab:
  host: https://gitlab.mydomain.nz
  token: abced871263876132jkd
groupsfilepath: $HOME/HM/labs/20WS
groups:
  - algdati
  - vss
```

### Group config file

Contents:

```
<baseNameOfGroup>:
    id: <group id>
```

Example:

```
algdati:
  id: 12345
```

## Structure of GitLab-Groups

-   Lecture
    -   Startercode
    -   Semester
        -   Semestername
            -   Assignment

## Usage

```
Manage GitLab for student assignments

Usage:
  glabs [command]

Available Commands:
  help        Help about any command
  show-config Show config of a group
  show-info   Show info for a group
  version     Print the version number of Glabs

Flags:
      --config string   config file (default is $HOME/.glabs.yml)
  -h, --help            help for glabs
  -v, --verbose         verbose output

Use "glabs [command] --help" for more information about a command.
```
