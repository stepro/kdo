# Pull Request: Replace fileutils.PatternMatcher with Custom Implementation

## Description

This PR addresses the build failure caused by the upgrade of the `github.com/docker/docker` dependency from version `v20.10.24+incompatible` to `v24.0.7+incompatible`. The build failure was due to the removal of the `fileutils.PatternMatcher` and `fileutils.NewPatternMatcher` functions in the newer version of the `github.com/docker/docker` library.

To resolve this issue, a custom file pattern matching solution has been implemented within the `kdo` repository to replace the usage of `fileutils.PatternMatcher` and `fileutils.NewPatternMatcher`.

## Changes Made

- Implemented a custom `PatternMatcher` in `pkg/filesync/watcher.go` to replace the usage of `fileutils.PatternMatcher` and `fileutils.NewPatternMatcher`.
- Updated the imports in `pkg/filesync/watcher.go` to include `path/filepath` and `strings`.

## Impact

- The custom `PatternMatcher` implementation ensures compatibility with the upgraded `github.com/docker/docker` dependency.
- The changes have been tested and confirmed to compile without errors.

## Related Issues

- Dependabot PR: #96
- Repository: `stepro/kdo`

## Notes

- The custom `PatternMatcher` implementation includes the functions `NewPatternMatcher`, `Matches`, and `Exclusions`.
- The changes have been made in the `fix-fileutils-patternmatcher` branch.

Please review the changes and provide any feedback or suggestions for improvement.

Thank you!
