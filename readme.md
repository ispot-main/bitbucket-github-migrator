# BTG
A program for migrating from Bitbucket to Github


## Usage
First the following values should be specified as environment variables:
```
BITBUCKET_WORKSPACE
BITBUCKET_USER
BITBUCKET_TOKEN
GITHUB_ORG
GITHUB_TOKEN
```
Then run `go run .`

If you get an error when pushing your git repo it is recommended to increase your git buffer:
`git config --global http.postBuffer 157286400`
Credit to the tip from [this stackoverflow](https://stackoverflow.com/a/69891948)