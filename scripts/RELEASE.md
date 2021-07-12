**To bump a new release of Tile38**

- Update CHANGELOG.md to include the newest changes.
- `git commit -m $vers` changes (where `$vers` is a semver)
- `git tag $vers`  (where `$vers` is a semver)
- `git push --tags`
- `git push` 
- `make package`
- Add a new Github Release and add the zips from packages directory.
 