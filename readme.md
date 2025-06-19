# BTG
A program for migrating from Bitbucket to Github


## Usage
First, create a file called `repos.txt` with the list of repos you want to migrate:
```
repoName1
repoName2
repoName3
```

Next, put your desired configuration in a `.env` file in the same directory as the executable.
For example:
```
# .env
BITBUCKET_WORKSPACE=YOUR_WORKSPACE_HERE
# you can see your username in https://bitbucket.org/account/settings/
BITBUCKET_USER=YOUR_USERNAME_HERE
BITBUCKET_TOKEN=CENSORED

GITHUB_ORG=YOUR_ORG_HERE
# You can use a PAT of a user, but make sure the token owner is the org
# The token MUST have write access to Administration, Contents, Issues, and Pull Requests
GITHUB_TOKEN=CENSORED

# whether overwriting existing github repo is allowed
GITHUB_OVERWRITE=false
GITHUB_DRYRUN=true

MIGRATE_REPO_CONTENTS=true
# it's suggested to migrate repo settings if you migrate repo contents
# as migrating repo contents may reset default branch
# and migrating repo settings will reset it back
MIGRATE_REPO_SETTINGS=true
MIGRATE_OPEN_PRS=true
# MIGRATE_CLOSED_PRS not quite ready for usage yet
MIGRATE_CLOSED_PRS=false

REPO_FILE=repos.txt
```
If you have the repo cloned locally, run `go run .`

If you have downloaded the executable, run the executable.

---

If you get an error when pushing your git repo it is recommended to increase your git buffer:
`git config --global http.postBuffer 957286400`

Credit to the tip from [this stackoverflow](https://stackoverflow.com/a/69891948)