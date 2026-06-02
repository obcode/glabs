## 2.9.0 (2026-06-02)

#### 🎁 Feature

* **startercode:** add optional tag for startercode in generated repositories (#70) (0bb2177d)


## 2.8.3 (2026-05-31)

#### 🐞 Bug Fixes

* **changelog:** simplify changelog update message format (919946ef)

## 2.8.2 (2026-05-31)

#### 🐞 Bug Fixes

* **release:** prepend changelog entries and restore history (6c67ac22)


## 2.8.1 (2026-05-31)

#### 🐞 Bug Fixes

* **release:** persist changelog updates and enable changelog-file (1c0af52e)


## 2.8.0 (2026-05-20)

#### 🎁 Feature

* add 'addgroupguests' command to manage student access for Dependency-Proxy (155c55e2)


## 2.7.0 (2026-05-05)

#### 🎁 Feature

* add support for including child tasks in issue replication (#69) (2e3dea67)


## 2.6.1 (2026-05-01)

#### 🐞 Bug Fixes

* encode also from branch in startercode url (0ff75c99)


## 2.6.0 (2026-05-01)

#### 🎁 Feature

* show urls of course and startercode (#68) (642c1c21)

#### 🔀 Code Refactoring

* refactor source repo preparation and push logic with new function signatures (bc6ee1ee)


## 2.5.0 (2026-04-30)

#### 🎁 Feature

* use startercode as template with one single commit (#67) (38b1e4c2)


## 2.4.1 (2026-04-30)

#### 🐞 Bug Fixes

* use committer name and email if set in glabs.yaml (#66) (d6879d79)


## 2.4.0 (2026-04-30)

#### 🎁 Feature

* add push command to handle deferred branches (#65) (9e4ba64b)

#### 📄 Documentation

* add useEmailDomainAsSuffix to example (70f5aa3f)


## 2.3.0 (2026-04-28)

#### 🎁 Feature

* add support for removing email domain suffix in repo naming (#64) (1cdb5789)

#### 📄 Documentation

* enhance installation instructions with PATH setup and prebuilt binaries (7486648b)
* update installation command to use v2 module path (72d8d48b)


## 2.2.0 (2026-04-26)

#### 🎁 Feature

* implement merge request approval rules and settings, closes #52 (#60) (caf96a73)


## 2.1.3 (2026-04-26)

#### 🐞 Bug Fixes

* enhance version command to display build information conditionally (#63) (a334911d)


## 2.1.2 (2026-04-26)

#### 🐞 Bug Fixes

* update module path to v2 for go install support (cd614fe1)

#### 🔀 Code Refactoring

* go install v2 (#62) (fa1642a3)


## 2.1.1 (2026-04-25)

#### 🐞 Bug Fixes

* branch protection for mergeOnly (#61) (b1b139b5)


## 2.1.0 (2026-04-23)

#### 🎁 Feature

* add additional branch protection flags, closes #51 (#59) (dc396cc3)


## 2.0.0 (2026-04-23)

#### 📣 Breaking Changes

* split startercode policy into branches and issues; apply branch rules during generate (#58) (a1aba847)
```

Refactor startercode configuration to separate branch and issue settings

- Migrate legacy `devBranch`, `protectToBranch`, and `protectDevBranchMergeOnly` keys to a new `branches` block for better clarity and separation of concerns.
- Introduce `issues` block for issue replication settings, moving from `startercode`.
- Update documentation to reflect new configuration structure and provide migration guidance.
- Implement branch protection and default settings directly within the `branches` configuration.
- Adjust related code in GitLab client to accommodate new structure, ensuring backward compatibility with legacy keys.
- Enhance tests to validate new branch protection logic and configuration handling.

Co-authored-by: Copilot <copilot@github.com>

BREAKING CHANGE: branch and issue policy moved from startercode to branches/issues. See `docs/migration.md`
```


## 1.3.0 (2026-04-23)

#### 🎁 Feature

* add merge request checks, see #54 (#57) (5e8abcfe)


## 1.2.0 (2026-04-23)

#### 🎁 Feature

* add squash option to merge requests, see #54 (#56) (4fbf7d11)

#### 📄 Documentation

* update configuration for merge request options and strategies (#55) (a11d68ba)


## 1.1.1 (2026-04-23)

#### 🐞 Bug Fixes

* releaser (#53) (a913106e)


## 1.1.0 (2026-04-23)

#### Feature

* set merge method to: merge | semi_linear | ff (#50) (9acf0492)

#### Code Refactoring

* migrate from xanzy/go-gitlab to gitlab.com/gitlab-org/api/c… (#48) (ba345930)

#### Tests

* add tests for glabs (#49) (90503d7f)


## 1.0.0 (2026-04-23)

#### ci

* Feature/fix ci (#47) (97ba55ea)

#### Documentation

* Add documentation for glabs commands, configuration, getting started, troubleshooting, and workflows (c37a0e0b)


## 0.29.0 (2026-04-22)

#### 🎁 Feature

* add protectDevBranchMergeOnly option to control dev branch merg… (#46) (b8eea871)


## 0.28.0 (2026-03-23)

#### 🎁 Feature

* add useCoursenameAsPrefix option to customize repository naming convention (#45) (f430c4f6)


## 0.27.0 (2026-03-12)

#### 🎁 Feature

* add functionality to replicate issues from startercode repository (114bdb0b)

#### 🐞 Bug Fixes

* remove replicateIssuesFromStartercode function to streamline issue replication process (a9baccc4)


## 0.26.0 (2026-03-10)

#### 🎁 Feature

* create subgroup aka assignmentpath in assignment-config if not existent (9a607e87)


## 0.25.0 (2026-01-27)

#### 🎁 Feature

* archive and unarchive projects (972a28e9)

#### 📄 Documentation

* git alias for starter code (25ade3da)


## 0.24.1 (2024-12-10)

#### 🐞 Bug Fixes

* Declare error variable and use assignment for spinner (#40) (546f07f3)

#### 🔀 Code Refactoring

* rm unused parameters (e5b720fc)
* for upgraded versions (8efbc5e1)

#### 🚧 Chores

* upgrade deps (fc128416)


## 0.24.0 (2024-11-05)

#### 🎁 Feature

* **urls:** urls without groups or students prints the assignment url (d5baa0d5)

#### 📄 Documentation

* **urls:** assignment url (90e90afd)


## 0.23.0 (2024-11-04)

#### 🎁 Feature

* add ulrs command (6f207073)


## 0.22.0 (2024-11-04)

#### 🎁 Feature

* **clone:** suppress output for piping (a48f63d3)


## 0.21.0 (2024-11-04)

#### 🎁 Feature

* students and groups on the command line are now regexps (bf9fe360)


## 0.20.2 (2024-11-04)

#### 🐞 Bug Fixes

* try semantic release (2f39855a)

#### 🚧 Chores

* prepare for semantic release (0ab87803)


## 0.20.1 (2023-09-11)

* Add `update` command.


## 0.19.0 (2023-03-08)

* 9259459 fix go releaser
* 7aa110e upgrade GitHub workflows
* c87de5b ✨ push additional branches, closes #29
* ce7a512 ⬆️ upgrade deps


## 0.18.2 (2022-11-29)

* b87ddbb fix protect branch and add protect command
* 26510f2 protect command
* 429e3d6 ⬆️ upgrade dependencies


## 0.18.1 (2022-10-29)

Add release information in the assignment config and see results in the report.

Example:

```yaml
...
  blatt3:
    per: group
    assignmentpath: blatt-3
    startercode:
      url: ...
      fromBranch: 22ws
      protectToBranch: true
      devBranch: develop
    release:
      mergeRequest:
        source: develop
        target: main
        pipeline: true
      dockerImages:
        - customer
        - book
...
```

## 0.18.0 (2022-10-28)

see https://github.com/obcode/glabs#generating-reports-of-assignments

## 0.17.2 (2022-10-22)

* fcb78be Merge branch 'release/v0.17.2'
* 530eca5 Merge tag 'v0.17.1' into develop
* 7a42112 🐛 fix a real stupid bug 🙈


## 0.16.1 (2022-08-10)

- add `setaccess` command to set access for existing projects

## 0.16.1

- just bump versions for GitHub CI actions


## 0.15.0 (2022-02-21)

- delete repos (#21 by @fritterhoff)
- use development branch (#22)

## 0.14.0 (2021-07-05)

Thanks to @fritterhoff for this one.

## Changelog

e7abca5 Allow usage of email addresses
0f6bd5c Merge branch 'release/v0.14.0'
7201078 Merge pull request #17 from fritterhoff/user-by-email
d86cbab Merge remote-tracking branch 'upstream/develop' into user-by-email
2d6a0a9 Merge remote-tracking branch 'upstream/develop' into user-by-email
efc4621 Merge tag 'v0.13.0' into develop
a7aeed9 Replace only @'s


## 0.13.0 (2021-07-05)

Thanks to @fritterhoff for this one.

## Changelog

0c5e589 Fix using other branch
bc11586 Merge branch 'release/v0.13.0'
2e4cbec Merge pull request #15 from fritterhoff/seeding
c6f9a35 Merge tag 'v0.12.0' into develop
fba128a Minor optimization in seeding
860f8d0 Optimize error handling and fix lint errors
a7f626d Provide documentation and an example for seeding
82439ad feat: add seeding functionality ✨
92af617 🔄 Update dependecy versions & ignore .idea
877a930 🛠 master to main in seeder component


## 0.12.0 (2021-05-09)

c62274e :boom: master -> main
795e213 Merge tag 'v0.11.0' into develop
97080b8 💥 change default branch to main


## 0.11.0 (2021-05-07)

2afb6a4 :memo: doc for local studs/groupd
2dbf680 :sparkles: studs/groups per assignment (appends)
837b3bb Merge branch 'feature/studsOrGroupsPerAssignment' into develop
89a6645 Merge tag 'v0.10.0' into develop
c6f6a38 info from addMember should be green
1f1bd21 studs/groups per assignment (appends)
86faa98 upgrade golangci-lint to 1.32


## 0.10.0 (2020-10-31)


## Changelog

5aa1bbc Merge branch 'release/v0.10.0'
713d8ba Merge tag 'v0.9.0' into develop
b3c3562 add -f to clone command



## 0.9.0 (2020-10-30)

Use ssh-agent by default or set `sshprivatekey` in `.glabs.yml`.

## Changelog

f81e72b Don't panic on missing arguments, typos
fc14148 Merge branch 'develop' of github.com:obcode/glabs into develop
bf6679d Merge branch 'release/v0.9.0'
745a8d3 Merge pull request #11 from JohannesEbke/sshagent
7ab5377 Merge pull request #12 from JohannesEbke/minorpanic
083791b Merge tag 'v0.8.1' into develop
dc5b3f1 Use ssh-agent unless gitlab.sshprivatekey is configured
4699d7c s/gitlab.sshprivatekey/sshprivatekey/



## 0.8.1 (2020-10-27)


## Changelog

b38f7b0 Merge branch 'release/v0.8.1'
66da0f7 Merge tag 'v0.8.0' into develop
c6b7192 add info in Readme about clone



## 0.8.0 (2020-10-27)

Clone Repos.

## Changelog

2f838b2 Merge branch 'release/v0.8.0'
f11e5a9 Merge tag 'v0.7.0' into develop
2302c8a clone repos



## 0.7.0 (2020-10-27)

Prettier output.
Changeable access level.

## Changelog

1df8d37 Merge branch 'release/v0.7.0'
787c0ca Merge tag 'v0.6.0' into develop
7963c9f better output in generate
97170d2 it is now possible to change the accesslevel
b39ce6d rename show() to String()



## 0.6.0 (2020-10-22)


## Changelog

d86fb4e Merge branch 'release/v0.6.0'
8f3f37a Merge tag 'v0.5.0' into develop
1610b3c s,ttacon/chalk,gookit/color,
b49609f show assignment config



## 0.5.0 (2020-10-21)


## Changelog

0c6a56c Merge branch 'release/v0.5.0'
5f2ac8f Merge tag 'v0.4.0' into develop
8cfd068 add config package
f3fab36 add course to assignment config
56d7ce8 check shows real name and uses config, close #10
decdd3b generate uses config
98eaa6f renames
ee5f4ae use defaults for golangci-lint



## 0.4.0 (2020-10-20)

It is now possible to generate for a subset ob configured groups or for some students only. Students do not need to be in the config file.

## Changelog

4c474ee Add students or groups to the generate command.
219dc12 Merge tag 'v0.3.1' into develop
7601a18 generate only for some groups or students



## 0.3.1 (2020-10-19)


## Changelog

298b06c Merge branch 'release/v0.3.1'
97d60c1 Merge tag 'v0.3.0' into develop
4985788 error if course config ist missing, Closes #7
a9f2fc2 find project if name is prefix, closes #6



## 0.3.0 (2020-10-18)

add configuration flag for enabling the GitLab container registry on repo creation

## Changelog

be904a5 Merge branch 'release/v0.3.0'
65471b6 Merge tag 'v0.2.0' into develop
266862f add flag for container registry
00c3878 log branches



## 0.2.0 (2020-10-17)

- Starter code can now be pushed from a different branch than `master`
- Starter code can now be pushed from a different branch than `master`
- Branch on GitLab can be protected
- `coursepath`, formerly known as `group` can now contain slashes

Breaking changes:

- renamed some groups in config files.

## Changelog

1cfa29b Merge branch 'feature/renameGLgroupToPath' into develop
b163909 Merge branch 'release/v0.2.0'
3326aca Merge tag 'v0.1.0' into develop
c60e538 add .yml to error message, closes #3
62bfeb1 add Caller to log
279af6c baseNameOfCourse
fbff54b more info in readme
ae323d8 rename groups (lectures) to courses
9cc2177 rename groups to course, path, group fixes #1 #4
0181330 startercode: fromBranch, toBranch, protectToBranch



## 0.1.0 (2020-10-11)


## Changelog

f5e9efd Initial
fcb20d1 Initial commit
30ce125 Initial release
ff13a2a Refactoring of package gitlab
9db85b7 add accesslevel as variable
df589f9 add check command
053df0a add check to readme
ad601fa add goreleaser
6e187f7 add info about per group generation
45f810f add support for semestergroup and assignemnt.group
557bd9c check if students are in more than one group
19603fa generate per group
8c9665e generate per user (without template)
c6f677d inject version et al by goreleaser
0dfe3de lint also on develop
69e5ec4 more debugging logs
f5f5355 remove show-info command
44e39fb remove template
f94a76c remove unused variable
16a2e61 run goreleaser only on tags
37527bb update Readme
562bfc4 update Readme
604b579 use startercode



